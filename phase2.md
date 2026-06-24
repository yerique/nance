# Phase 2: Read-through Cache

**Goal**: Layer a Redis-backed read-through result cache on the Phase 1 passthrough proxy. When a tenant has declared a collection as cacheable, qualifying read operations (`find`, certain `aggregate`, `count`, etc.) are served from Redis when possible. Cache population happens on miss. Writes to cacheable collections trigger invalidation. The system remains safe and correct even when Redis is unavailable (fail open to passthrough).

This phase turns the accelerator from a pure pooler into a true performance multiplier for read-heavy workloads.

## Objectives & Success Criteria

- Per-tenant, per-namespace cache policy (loaded from control plane storage) drives decisions.
- Supported cacheable operations produce identical results whether served from cache or from the real backend.
- Cache keys are deterministic and stable for the same logical query (normalization + hash).
- On cache hit: client receives a correct MongoDB reply (results + cursor metadata) with very low added latency.
- On cache miss for a cacheable read: execute against backend (reuse Phase 1 logic), store the result set in Redis (subject to size limits), then return to client.
- TTLs are honored exactly as configured per collection (or tenant default).
- Writes (insert/update/delete/bulk etc.) to a cache-enabled collection cause the cached entries for that namespace to be flushed (collection-level invalidation).
- Transactions that contain reads bypass the cache for the duration of the transaction.
- Single-flight / request coalescing on miss: N concurrent identical cache misses for the same key result in only **one** real database query.
- Size guardrails prevent huge result sets from being cached (per-collection `maxResultBytes` or global default).
- When Redis is down or slow, all traffic degrades gracefully to pure passthrough (Phase 1 behavior) with no errors surfaced to clients for cache-related reasons.
- Metrics: hit rate, miss latency, cache fill size, invalidation counts, per-tenant breakdowns.
- No data leakage between tenants in the cache (keys are strongly tenant-prefixed).

## Key Design Elements from Architecture (recap + refinement)

**Cache key** (example):
```
nance:tenant:{tenantId}:ns:{db}.{coll}:cmd:{sha256_of_normalized_command}:v{cacheKeyVersion}
```
- Use Redis hash tags `{tenantId}` so all keys for a tenant hash to the same slot in a Redis Cluster. This enables efficient per-tenant operations.
- Normalization is **critical** for hit rate:
  - Remove non-semantic fields: `$comment`, `comment`, `maxTimeMS` (unless you want to vary), read preference fields that don't affect results, `$readPreference`, `hint` in some cases?
  - Canonicalize filter/projection/sort objects: recursively sort keys.
  - Represent numbers consistently (BSON int32 vs int64 vs double can matter – decide on a canonical form).
  - Include the command name itself (`find` vs `aggregate`).
  - The full pipeline for aggregate is part of the shape.
- Store in Redis: a binary value (BSON of the result batch(es) + cursor metadata + original command hash for debugging + timestamp). Use `SET key EX ttl` or `SET ... EXAT`.

**Value stored**:
- Prefer storing the **result documents as BSON bytes** (array of raw docs) plus a small header describing `cursorFirstBatch`, flags, etc.
- On hit, reconstruct a minimal but valid reply document the wire layer already knows how to send.
- Alternative: store the complete wire reply document. Tradeoff is size vs reconstruction simplicity.

**Policy lookup**:
- Fast in-memory cache of the tenant's full `CachePolicy` (refreshed periodically or on explicit notification).
- Collection key matching: exact `db.coll` first. Future phases can add `db.*` and `*.coll` patterns.
- Default: if no entry, use `defaultTtlSeconds` or treat as disabled (recommendation: disabled by default for safety – tenant must explicitly enable).

**Invalidation on write** (collection prefix / namespace level):
- After a successful mutating command on `db.coll`:
  - If that ns has caching enabled, trigger flush of all keys under the prefix for that (tenant, ns).
- Implementation options (pick one primary for Phase 2, the other as future improvement):
  1. **Registry Set** (recommended for Phase 2): Maintain a parallel Redis set `nance:tenant:{t}:ns:{db}.{coll}:known_keys` (also hash-tagged). Every time we do a successful `SET` for a cache entry we also `SADD` the cache key into the set. On write: `SMEMBERS` the set, pipeline `DEL` for each + `DEL` the set itself. Do this in a background goroutine or with bounded batching + timeout so writes are not slowed.
  2. **SCAN + MATCH + DEL**: On write, run a bounded `SCAN MATCH "nance:tenant:{t}:ns:db.coll:*" COUNT 1000` repeatedly until cursor 0, issuing `DEL`. Risk: slow on large key counts; use with care and in background.
- TTL-only is acceptable as a starting point if the registry approach takes too long, but the architecture recommends moving quickly to collection-level flush on write.

**Cache bypass rules** (hard requirements):
- Any command inside an active transaction (`lsid` present + `txnNumber`).
- Non-cacheable commands by nature: all writes, `getMore` on a real (non-cached) cursor, `killCursors`, change streams (`$changeStream`), tailable cursors, `aggregate` containing `$out` / `$merge` / `$changeStream`.
- Commands with `$natural` sort? (often want fresh data).
- Very large limits or no limit + large result (enforced by `maxResultBytes` check after execution on miss).
- Any command where we cannot produce a deterministic cache key (unknown shape).
- Explicit bypass flag if we later add `$comment` or a side channel.

## Detailed Implementation Steps

1. **Redis Client Integration**
   - Add `github.com/redis/go-redis/v9` (or `rueidis` for higher perf / pipelining if desired).
   - Config: address(es), password, DB index, TLS, pool size, timeouts, retry policy.
   - Provide a thin `Cache` interface in `internal/proxy/cache/` with methods:
     - `Get(ctx, key string) ([]byte, error)`
     - `Set(ctx, key string, value []byte, ttl time.Duration) error`
     - `InvalidateNamespace(ctx, tenantID, db, coll string) error` (or more granular)
     - `Health()` or used in readiness.
   - Support Redis Cluster and single instance from day one (the hash tag design works for both).
   - Connection is per-proxy (stateless); all proxies share the same Redis.

2. **Policy Engine**
   - `PolicyEngine` component that owns a map or sync.Map of `tenantID -> *model.CachePolicy`.
   - Loader: on startup + on a ticker (e.g. every 30s) or via Postgres LISTEN/NOTIFY or a simple "last updated" poll, refresh policies for active tenants.
   - Or simpler: on every command do a cheap Postgres read (or cache the whole policy row in Redis with short TTL). Prefer in-memory hot cache + periodic refresh.
   - Expose `IsCacheable(tenantID, db, coll string, cmdName string) (bool, time.Duration, int /*maxBytes*/)`
   - Version the policy (the `cacheKeyVersion` field). Include it in cache keys so a policy change that affects normalization can naturally miss old keys.

3. **Cache Key Generator (pure, well-tested module)**
   - `func CacheKey(tenantID, db, coll string, cmdName string, cmd bson.Raw, cacheKeyVersion int) (string, error)`
   - Steps inside:
     - Parse to `bson.D` or walk `bson.Raw`.
     - Deep-copy and mutate to produce a "normalized" command:
       - Delete volatile top-level keys.
       - Recursively sort all `bson.D` / `bson.M` objects by key (stable sort).
       - Canonicalize numeric types if desired (e.g. promote int32 to int64 for comparison? decide and document).
       - For aggregates: keep the pipeline array as-is (after sorting objects inside stages).
     - Serialize the normalized form to bytes (use `bson.Marshal` or a deterministic JSON form for the hash input).
     - `sha256(normalized + "|" + cmdName + "|v" + version)`
     - Return the full Redis key string.
   - Write extensive unit tests with table-driven cases: different key order in filter should collide; `$comment` should not affect key; different `limit` values produce different keys, etc.
   - Add a small debug mode that can log the normalized shape (redacted) when `NANCE_DEBUG_CACHE_KEYS=1`.

4. **Cache Coordinator / Read Path (inside the command handling loop from Phase 1)**

   Pseudocode (per cacheable read command after namespace + policy lookup):

   ```go
   if !policy.Enabled { goto passthrough }

   key, _ := cacheKeyGen(...)

   // singleflight per key to collapse concurrent misses
   resultBytes, err := singleflight.Do(key, func() ([]byte, error) {
       b, err := redis.Get(ctx, key)
       if err == nil { return b, nil } // hit
       if !errors.Is(err, redis.Nil) { /* redis error - fail open */ return nil, errPassthrough }

       // MISS
       reply, err := executeOnBackend(tenantCtx, cmd)  // reuses Phase 1 path
       if err != nil { return nil, err }

       serialized := serializeForCache(reply) // BSON of docs + metadata
       if len(serialized) > policy.MaxResultBytes { return nil, errTooBigForCache }

       _ = redis.Set(ctx, key, serialized, policy.TTL)  // best effort
       // also SADD to the ns known_keys set (best effort)
       return serialized, nil
   })

   if resultBytes != nil {
       // HIT or we just populated
       sendCachedReplyToClient(wireConn, resultBytes)
       recordHitMetric()
       return
   }
   // fall through to normal passthrough on any cache error
   passthrough(...)
   ```

5. **Single-flight / Request Coalescing**
   - Use `golang.org/x/sync/singleflight` keyed by the cache key (or a tenant-scoped variant).
   - This is one of the highest-ROI pieces for stampede protection.
   - The `Do` func above does the Redis GET + possible backend query + SET.

6. **Write Path & Invalidation**
   - In the existing write command handling (after successful execution against backend):
     ```go
     if policy.EnabledForThisNs {
         go func() { // or a bounded worker pool
             _ = cache.InvalidateNamespace(tenantID, db, coll)
         }()
     }
     ```
   - Implement `InvalidateNamespace` using the registry-set strategy.
   - Provide an explicit invalidation API (control plane can call an internal endpoint on proxies, or proxies can react to a Redis pub/sub "invalidate" channel, or simply let the registry set be the source of truth).
   - Also expose a control-plane HTTP endpoint `POST /tenants/{id}/invalidate?db=..&coll=..` that proxies can poll or that directly talks to Redis (if control plane has Redis access).

7. **Result Serialization for Cache**
   - Decide on a small struct:
     ```go
     type CachedResult struct {
         Docs         []bson.Raw   // or []byte (concatenated with length prefix)
         CursorID     int64        // usually 0 for cached
         NS           string
         // other flags
     }
     ```
   - On hit path in the wire handler: turn `CachedResult` back into the exact reply shape a live `find` or `aggregate` would have produced (`{ ok:1, cursor: {id:0, ns:.., firstBatch: [...] } }`).
   - For very small results we can return everything in the first batch and set `id:0`. This is the simplest and sufficient for the vast majority of cached use cases (`toArray()`, small pages).

8. **Cursor Handling for Cached Results (MVP)**
   - Primary strategy (Phase 2): **Always return `cursor.id = 0`** for cache hits and put the entire result in `firstBatch`.
   - This works beautifully when the application does `.limit(N).toArray()`, aggregation that fits in memory, counts, etc.
   - If a driver immediately follows with a `getMore` on a cursorID we returned as 0, treat it as an error or empty (should not happen if we set id=0 correctly).
   - Full server-side cursor emulation for cached results (holding the docs in the proxy process memory with a timeout) can be added late in Phase 2 or in Phase 3 if real usage shows the need. Document the limitation: "Cached reads are returned as complete batches with no server cursor."

9. **Cache Bypass & Edge Cases**
   - Detect transaction: if the command document or the OP_MSG has `lsid` + `txnNumber` (and not the "autocommit" only case), skip cache entirely for the whole txn.
   - After a write inside a txn that commits, the post-commit invalidation should still fire (the write handler sees the successful commit).
   - Handle `batchSize` on the original query: when we cache we can ignore client batchSize for storage and always give the full set on hit (the client driver will handle slicing if needed, or we can respect it on reconstruction – over-delivering is usually fine).
   - Projections: they are part of the cache key (different projection = different key). This is correct.
   - `explain`? Usually not cached (passthrough).

10. **Degradation & Resilience**
    - Wrap every Redis call with short timeouts (5-50ms for GET on hot path).
    - On any Redis error (connection, timeout, auth, OOM, etc.) during read path: increment "redis_unavailable" metric and fall straight through to `executeOnBackend`.
    - Never let a cache SET failure after a successful miss turn into a client error. Fire-and-forget the SET.
    - Circuit-breaker around Redis is nice-to-have (simple counter of recent failures).
    - On proxy startup, do a non-fatal Redis ping; log but continue (fail-open philosophy).

11. **Metrics & Observability Additions**
    - `nance_cache_hits_total{tenant, ns, command}`
    - `nance_cache_misses_total{...}`
    - `nance_cache_latency_seconds` (separate histograms for hit path vs miss path)
    - `nance_cache_result_bytes` (size of what we stored)
    - `nance_cache_invalidations_total{tenant, ns, reason="write"}`
    - `nance_cache_bypass_total{reason="transaction|size|..."}`
    - Per-tenant Redis roundtrip time sampled.
    - Expose current policy version per tenant in metrics or `/debug` endpoint.

12. **Testing Strategy for Phase 2 (much more than Phase 1)**

    - Unit tests:
      - Normalization + key generation (dozens of cases, including tricky pipelines and filters with `$in`, `$or`, nested objects, key ordering).
      - Size guard logic.
      - Singleflight behavior (use a slow fake backend to prove only one execution).

    - Integration tests (testcontainers: Postgres + Mongo + Redis):
      - Create tenant, set real backend (the container mongo), enable caching on `mydb.orders` with 5s TTL.
      - From a driver client pointed at the proxy:
        - Perform a `find` → observe miss + real query happened (can assert via Mongo server logs or a slow query collection or by counting commands on the backend client).
        - Immediate repeat of identical `find` → cache hit, no additional load on Mongo (prove by using a unique slow `$comment` or by checking Redis directly, or by asserting backend query count stayed flat).
        - Mutate the collection → verify subsequent read is a miss (or at least that the old cached value is gone).
        - TTL test: write something, wait > TTL, read again → miss + fresh value.
      - Concurrent 50 identical misses → assert only 1 backend execution.
      - Transaction test: start txn, do reads inside txn on a cacheable collection → all go to backend.
      - Redis kill during test: proxy keeps working (passthrough), no client-visible errors from cache path.
      - Different tenants with same db.coll names never collide in Redis (key inspection).

    - Chaos: restart Redis while traffic is flowing; restart proxy (stateless, should recover).

    - Property test idea (optional): generate random simple filters, run against real Mongo, run through proxy with cache on, assert equality of result sets.

13. **Configuration & Policy Surface**
    - Control plane API from Phase 0 is already sufficient.
    - Proxy must react to policy changes reasonably quickly (within tens of seconds via refresh loop is fine for Phase 2; immediate via pubsub is Phase 3 polish).
    - Changing a collection from `enabled:false` → `true` should start caching new queries.
    - Changing TTL should affect new cache writes (existing entries keep their original TTL until they naturally expire or are invalidated).

14. **Performance Considerations**
    - Keep the hot cache-hit path extremely lean: one Redis GET (ideally < 1ms), minimal CPU for key calc + reply reconstruction, then wire write.
    - BSON work: try to keep raw bytes where possible rather than full unmarshal/re-marshal on the hit path.
    - For very high throughput, consider client-side sharding of the Redis keyspace or multiple Redis instances later, but one well-sized cluster is the target for Phase 2.

## Deliverables

- Working read-through cache for the declared set of cacheable collections.
- Correct invalidation on writes using a registry-set or equivalent strategy.
- Single-flight on miss.
- Fail-open behavior proven.
- Rich metrics and good test coverage (especially normalization and consistency).
- Updated driver compatibility matrix showing that cached reads produce identical results to direct/passthrough.
- Documentation: "When to enable caching", "Consistency model", "How to choose TTLs", "How writes affect cache".
- A small "cache savings" example in the README (e.g. "under this load we reduced Mongo QPS by 87%").

## Risks & Mitigations

- **Risk**: Normalization is not perfect → wrong cache hits (stale or incorrect data for semantically different queries).  
  **Mitigation**: Extremely thorough table-driven tests + round-trip property tests against real Mongo results during development. Make the normalizer conservative (when in doubt, different key).
- **Risk**: Invalidation is racy or incomplete under high write load.  
  **Mitigation**: Invalidation is best-effort "flush what we know". Short TTLs are the safety net. Document the staleness window explicitly.
- **Risk**: Caching huge or long-running queries accidentally.  
  **Mitigation**: Hard `maxResultBytes` check on the populated result before the SET. Also consider adding a `maxDocs` or execution time guard. Uncached collections remain the escape hatch.
- **Risk**: Redis memory pressure or OOM killing cache value.  
  **Mitigation**: Configure Redis with `maxmemory-policy volatile-lru` or `allkeys-lru`. The per-collection size limit helps. Monitor `evicted_keys`.

## Open Questions for Phase 2

- Exact stored representation (full reply vs minimal result docs + reconstruction code) – prototype both for size/latency.
- Whether to support limited `getMore` against cached results within the same proxy process (adds state) or strictly `id:0` results.
- How aggressively to refresh policy in the proxy (poll interval vs evented).

**Exit checklist** (examples):
- [ ] Identical `find({status:"shipped"}).sort({created:-1}).limit(20)` returns byte-for-byte equivalent results from cache vs real (modulo order of fields that don't matter).
- [ ] 1000 QPS identical read workload against a cached collection results in << 10 QPS against the real Mongo after warm-up.
- [ ] A write to the collection makes the next read go to the database (invalidation visible within 1 second).
- [ ] Killing Redis container does not cause application errors; traffic continues at passthrough latency.

**Next phase preview**: Phase 3 focuses on production hardening — better cursor support for cached hits, deeper observability, rate limiting/quotas, multi-region considerations, explicit invalidation APIs, performance tuning, and optional client SDK sugar.

---
*Phase 2 is where the majority of the "accelerator" performance benefit materializes.*
