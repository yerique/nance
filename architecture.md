# Nance Accelerator Architecture

**Status**: Draft v0.1 — Architecture definition only. No implementation yet.

**Date**: 2026-06-17

## 1. Goals & Non-Goals

### Primary Goals
- Act as a **transparent proxy** in front of MongoDB clusters for multiple tenants.
- Enable **read-through caching** (Redis-backed) for selected collections on a **per-tenant, per-collection** basis.
- Use a **connection string** convention (`mongodb+nance://...`) so that applications and tools can connect to Nance instead of (or in addition to) raw MongoDB.
- Deliver **connection pooling benefits** similar to Prisma Accelerate without requiring changes to application query code.
- Give tenants **declarative control**: they configure *which* collections get caching and the TTLs; the proxy enforces the policy automatically.
- Uncached collections/namespaces must **directly hit the real MongoDB** (passthrough) with minimal overhead.
- Preserve full MongoDB driver semantics, pooling behavior (from the client's perspective), cursors, auth, etc. as much as possible.
- Provide strong **tenant isolation** (data, connections, credentials, rate limits, cache).

### Inspiration: Prisma Accelerate
Prisma Accelerate provides:
- A special `prisma://` (or `prisma+postgres://`) connection string that routes all traffic through their global edge + regional pooler service.
- Built-in connection pooling (solves serverless "too many connections" problem).
- Optional **per-query** `cacheStrategy: { ttl, swr?, tags? }` for read-through result caching at the edge.
- Writes and uncached reads go through the pooler to the real DB.
- The client (via `@prisma/extension-accelerate`) speaks an HTTP-based protocol to Accelerate; the real DB driver connection is never made from the app runtime.

**Nance differences (intentional)**:
- Native MongoDB focus (not tied to an ORM).
- **Collection-level declarative caching configuration** managed centrally by the tenant (no need to annotate every query in application code).
- Uses Redis explicitly as the cache backend (user requirement).
- Aims for high driver compatibility via a MongoDB-wire-protocol-speaking proxy (or very close equivalent) so existing `MongoClient` usage, `mongosh`, Compass, drivers in any language, etc. continue to work with only a URI change.
- "mongodb+nance" connection string scheme (user-specified analogy).

### Non-Goals (for initial architecture)
- Full global anycast edge PoP deployment on day one (can be added later).
- Caching of change streams, tailable cursors, or real-time notification use cases.
- Acting as a full replica set / sharded cluster topology emulator (present as a single endpoint).
- Write-through caching or complex materialized views.
- Cross-tenant shared collections or federated queries.
- Being a replacement for MongoDB Atlas Search, BI connectors, etc.

## 2. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Client Applications / Tools                  │
│   (Node, Python, Java, mongosh, Compass, BI tools, etc.)            │
│   Connection string: mongodb+nance://tenantId:token@accel.nance.dev/db
└─────────────────────────────────────────────────────────────────────┘
                                   │
                                   │ MongoDB Wire Protocol (preferred)
                                   │ (or Nance SDK over HTTP/gRPC)
                                   ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        NANCE ACCELERATOR (Data Plane)                │
│  - Proxy fleet (stateless, horizontally scalable)                    │
│  - Tenant resolver + AuthN/AuthZ                                     │
│  - Command inspector (extract ns = db.collection + op type)          │
│  - Cache Policy Engine (lookup per-tenant collection config)         │
│  - Read-through cache coordinator                                    │
│  - Passthrough engine                                                │
│  - Cursor / response shaper                                          │
└─────────────────────────────────────────────────────────────────────┘
          │                                   │
          │ Cache path (Redis)                │ Passthrough / Miss path
          ▼                                   ▼
┌──────────────────────┐            ┌────────────────────────────────────┐
│   Redis Cluster      │            │   Per-Tenant Mongo Pool Manager    │
│  (result cache)      │            │   - One MongoClient per tenant     │
│  - Keys by tenant+ns │            │   - Real connection strings (encrypted)
│  - TTL + LRU         │            │   - Proper pooling, timeouts, TLS  │
│  - Shared across     │            │   - Health checks, circuit breakers│
│    proxy instances   │            └────────────────────────────────────┘
└──────────────────────┘                        │
                                                │ MongoDB Wire Protocol
                                                ▼
                                      ┌──────────────────────────────┐
                                      │   Tenant's Real MongoDB      │
                                      │   (Atlas, self-hosted,       │
                                      │    DocumentDB, etc.)         │
                                      └──────────────────────────────┘

Control Plane (separate or co-located HTTP/gRPC API):
  - Tenant onboarding & config CRUD
  - Store encrypted Mongo connection strings + cache policies
  - Invalidation APIs, usage metrics, audit
  - Secrets management integration (KMS)
```

**Key property**: From the application's point of view, Nance *is* the MongoDB server for the duration of the connection string. The app's driver opens connections/pools against Nance. Nance maintains its own pools to the real backend(s).

## 3. Connection String Format

Recommended primary form (enables drop-in replacement):

```
mongodb+nance://<tenant-id>:<api-token>@<nance-host>[:port]/<default-db>?authSource=admin&...other-options...
```

Examples:
- `mongodb+nance://proj_abc123:sk_live_9f3k...@accelerator.nance.dev/myappdb?retryWrites=true&w=majority`
- `mongodb+nance+srv://...` if we provide DNS SRV for load-balanced discovery.

**Parsing & resolution (in proxy)**:
- The scheme `mongodb+nance` (and variants) signals "this client intends to speak to a Nance accelerator".
- `username` = tenant identifier (stable, URL-safe id).
- `password` = short-lived or long-lived credential (API key / token) issued by Nance control plane. Never the real Mongo password.
- Host is always a Nance-controlled endpoint (never the real cluster hosts).
- The real database name can be carried through or overridden in config.

**Alternative / fallback form** (if full wire protocol is delayed):
- The client uses a Nance SDK (`@nance/accelerator-client` or similar) that accepts a similar string (or explicit `tenantId + token + endpoint`) and translates high-level operations into Nance's internal query protocol (HTTP or gRPC + BSON payloads). This is much easier to implement initially but requires language-specific SDKs.

**Strong recommendation**: Invest in **MongoDB wire protocol compatibility** as the primary integration path. This gives the Prisma-Accelerate-like "just change the URL" experience with zero SDK dependency and maximum tool compatibility. The cost is higher initial implementation complexity.

## 4. Core Components

### 4.1 Control Plane
- Tenant registry.
- Storage for:
  - Encrypted real `MONGODB_URI` / connection options per tenant (or per logical cluster).
  - Per-tenant cache policy map: `{ "db.collection": { enabled: true, ttlSeconds: 300, maxResultBytes?: 1_000_000 }, ... }` + tenant-wide defaults.
- Credential issuance & rotation for `mongodb+nance://` tokens.
- Invalidation endpoints (`/invalidate?tenant=...&collection=users` or by tags if we later support query tags).
- Validation: on save, attempt a lightweight connection test (ping + listCollections) using the supplied real URI. Never store plaintext URIs.
- Audit log of config changes and invalidations.
- Possibly a dashboard (later).

**Secrets handling**:
- Real Mongo connection strings are highly privileged.
- Use envelope encryption: data key (DEK) per tenant or per cluster, wrapped by a root KMS key (AWS KMS / GCP KMS / Vault transit).
- Proxy instances receive only *decrypted-at-runtime* credentials via short-lived leases or a sidecar / secrets injection.
- Never log connection strings or full auth payloads.

### 4.2 Data Plane / Proxy (The Accelerator Core)
Stateless (or soft-state) processes that terminate client connections.

Responsibilities:
1. Accept TCP connections speaking MongoDB wire protocol (focus on OP_MSG + modern commands).
2. Perform handshake (hello / isMaster responses — present a simple single-node topology).
3. Authentication:
   - Support at least one mechanism (SCRAM-SHA-256 recommended for compatibility, or a custom SASL mechanism that is just "tenant token").
   - On success, the connection is bound to a tenant context.
4. For every subsequent command:
   - Parse enough of the command to identify namespace (`$db` + collection from `find` / `aggregate` / `insert` etc. command documents).
   - Resolve cache policy for `db.coll` (or `db.*` / `*` wildcards if supported).
   - **If policy says cached (and operation is cacheable read)**:
     - Compute deterministic cache key.
     - Check Redis.
     - Hit → reconstruct and return a valid MongoDB reply (single batch or synthetic cursor).
     - Miss → execute against real backend (using the tenant's pooled client), store result (if within size/allowance), return to client.
   - **Else (passthrough or non-cacheable)**:
     - Forward / re-issue the command on the tenant's real MongoClient.
     - Stream results back (for large cursors, avoid full materialization when possible).
5. Handle writes/mutations:
   - Always forward to real backend.
   - On success for a collection that has caching enabled → trigger invalidation (see section 7).
6. Transaction context detection: if `lsid` + `txnNumber` present and active, prefer passthrough + bypass cache for the duration of the txn (or at minimum for reads inside the txn).
7. Connection lifecycle: respect client disconnects, kill cursors, etc. Maintain mapping of client cursors to real backend cursors (when not cached).

**Command classification (MVP)**:
- **Cacheable reads** (subject to policy): `find`, `aggregate` (pipelines without `$out`/`$merge`/`$changeStream`), `count`, `estimatedDocumentCount`, `distinct`, `listCollections` / `listIndexes` (maybe).
- **Always passthrough**: writes (`insert`, `update`, `delete`, `bulkWrite`, `findAndModify` that mutates, `createCollection`, admin commands, `hello` after initial, `killCursors`, `getMore` for real cursors), change streams, tailable cursors.
- Unknown / new commands → passthrough with warning log.

**Cursor handling on cache**:
- On miss for a cacheable query, the proxy exhausts the cursor (or up to a configured max docs / bytes) into memory, caches the full result set + original command shape metadata.
- On hit, return the results as a first batch with `cursor.id: 0` (no server-side cursor) or emulate a short-lived cursor if the driver will issue `getMore`.
- This works extremely well for queries that do `.limit(N).toArray()` or small result sets. Large unbounded scans should either not be cached or have policy limits.

### 4.3 Cache Layer (Redis)
- Used purely as a **read-through result cache**.
- **Key structure** (example):
  ```
  nance:tenant:{tenantId}:ns:{db}.{coll}:cmd:{sha256_of_normalized_command}:v1
  ```
  - Use Redis hash tags `{tenantId}` if using Redis Cluster so all keys for a tenant live on one slot (enables efficient per-tenant invalidation via SCAN + DEL in a pipeline or Lua).
  - Normalize the command for hashing: sort object keys where order doesn't matter (filters, projections), remove non-deterministic fields (comment, `$comment`, read preferences that don't affect results, etc.), canonical BSON or stable JSON.
  - Include a small version or "cache key schema version" to allow future evolution.

- **Value**: The MongoDB wire reply document(s) (or a compact internal representation: array of BSON docs + cursor metadata + response flags). Store as binary (BSON bytes) or CBOR for compactness. Also store original command hash + timestamp for debugging.
- **TTL**: Exact value from the collection policy `ttlSeconds`. Use Redis `SET ... EX` or `EXAT`. No sliding expiration unless explicitly added later.
- **Eviction & memory**: Configure Redis with `maxmemory-policy allkeys-lru` or `volatile-lru`. Set per-tenant or global max result size to avoid caching huge scans. Optionally add a `max_cached_result_bytes` per-collection policy.
- **Consistency**: Eventual. Cache is not strongly consistent across regions unless you run a single Redis (or strongly consistent Redis Enterprise setup). Per-PoP / per-region caches are acceptable (like Prisma).
- **Metrics exported**: hit/miss ratio per tenant/collection, latency of cache path vs miss path, bytes served from cache, eviction count.

**When Redis is unavailable**: Fail open to passthrough (execute on real Mongo). This preserves availability at the cost of load on the tenant DB. Circuit-breaker or degradation mode is important.

### 4.4 Backend Connection Pool Manager
- Maintains a map: `tenantId -> mongo.Client` (or connection pool handle).
- On first use (or lazy), create a `MongoClient` using the tenant's decrypted real connection string.
- Configure pool settings:
  - `maxPoolSize`, `minPoolSize`, `maxIdleTimeMS`, `waitQueueTimeoutMS` — either sensible global defaults or tenant-overridable in their config.
  - TLS, auth mechanism, replica set awareness, read preference, etc. come from the stored URI.
- The pool lives in the accelerator process(es) — **this is the source of the pooling benefit**. Dozens or hundreds of short-lived app serverless invocations all funnel their "connections" into a small number of long-lived Nance-to-Mongo connections per tenant.
- Health: background monitors, failover handling (the real driver already does most of this).
- Isolation: a bug or noisy neighbor in one tenant's pool must never affect another's client object.
- Connection string rotation: support updating the real URI without dropping active proxy connections (new commands after update use the new client; drain old).

### 4.5 Auth & Tenant Resolver
- Early in the connection or per-command (for stateless designs), resolve the tenant and validate the credential against control plane (or a fast cache of issued tokens with revocation list).
- Once resolved, all subsequent operations on that connection (or request) are executed in the context of that tenant + their policy + their backend pool.
- Support connection-string level credentials and also header / metadata based for non-wire protocols.

## 5. Read Flow (Cached Collection)

```
Client (driver)                    Nance Proxy                  Redis               Tenant Mongo Pool
     |                                 |                          |                       |
     |--- OP_MSG { find: "orders", ... } -->|                      |                       |
     |                                 |  1. Auth/tenant context  |                       |
     |                                 |  2. Lookup policy: orders -> cached, ttl=120s
     |                                 |  3. Compute cache key    |                       |
     |                                 |------ GET key ------------->|                      |
     |                                 |<----- HIT (BSON results) ---|                      |
     |<-- OP_MSG reply (docs) ----------|                           |                       |
     | (fast path, low latency)        |                           |                       |

Cache MISS path:
     |                                 |                           |                       |
     |                                 |  3b. Miss                |                       |
     |                                 |  4. Use tenant's pooled MongoClient
     |                                 |------ find(...) ---------------------------------->|
     |                                 |<----- [docs] -------------------------------------|
     |                                 |  5. If size ok: SET key EX ttl (results)
     |                                 |  6. Return results to client
     |<-- reply ------------------------|
```

Writes always go tenant Mongo (and may cause invalidation afterward).

## 6. Cache Policy & Configuration Model

Example tenant cache config (stored in control plane, exposed via API):

```json
{
  "tenantId": "proj_abc123",
  "realMongoUriEncrypted": "<envelope...>",
  "defaultTtlSeconds": 60,
  "collections": {
    "myappdb.users": {
      "enabled": true,
      "ttlSeconds": 300,
      "maxResultBytes": 5242880
    },
    "myappdb.products": {
      "enabled": true,
      "ttlSeconds": 3600
    },
    "myappdb.orders": {
      "enabled": false
    },
    "myappdb.sessions": {
      "enabled": true,
      "ttlSeconds": 30
    }
  },
  "cacheKeyVersion": 1,
  "updatedAt": "2026-06-17T12:00:00Z"
}
```

Collection keys can be:
- Exact `db.collection`
- Or patterns later (`db.*`, `*.auditLogs`)

Tenants manage this via control plane API / UI. Changes take effect for *new* queries (in-flight can be ignored or use a config version check).

## 7. Writes & Invalidation

Strategy tiers (implement progressively):

1. **TTL only** (MVP safe): No write-triggered invalidation. Correctness relies on short-enough TTLs. Simple and robust.
2. **Collection prefix invalidation** (recommended early addition):
   - On any successful write command targeting `db.coll` where caching is enabled for that coll:
     - After the write succeeds, issue a Redis command to delete all keys under `nance:tenant:{t}:ns:{db}.{coll}:*`.
     - Implementation: maintain a Redis Set per (tenant, ns) of "known cached keys for this ns", or use `SCAN` with `MATCH` + `DEL` (careful with production load; do it in background or with bounded batches).
     - Alternative: store cache entries in Redis Hashes or use a secondary index.
3. **Smarter invalidation** (future):
   - When caching a read, also record a "query fingerprint" (e.g. the filter shape).
   - On write, attempt to match which cached queries' filters would have been affected (very hard in general; Mongo filters are rich).
   - Or support explicit "tags" in the collection policy or per-query (if we add a side-channel). Tenants call an explicit invalidate-by-tag API after important writes.
4. **Hybrid**: short global TTL + explicit invalidation API + collection flush on writes for high-value collections.

Recommendation: Start with **TTL + collection-level flush on write** (tier 2). Document that reads may be stale up to the TTL window (or until next write-triggered flush).

Transactions that do reads-then-writes inside the same txn must bypass cache for the reads.

## 8. Connection Pooling Model (Two Layers)

- **Layer 1 — App to Nance**:
  - The application (or serverless function) configures its Mongo driver with the `mongodb+nance://...` URI.
  - The driver creates its normal connection pool (e.g. 5–100 connections depending on concurrency) **pointed at Nance**.
  - Nance appears as a fast, always-available MongoDB endpoint. This layer gives the *application* the pooling semantics and retry behavior it expects.

- **Layer 2 — Nance to Real Mongo** (the valuable pooling):
  - Nance holds a small number of persistent connections (or a sized pool) to the tenant's actual cluster using the stored real URI.
  - All cache misses + all passthrough traffic + all writes from *all* application instances of that tenant share these backend connections.
  - This prevents connection exhaustion on the real DB (especially important for Atlas free/shared tiers, serverless apps, high scale).

This is directly analogous to Prisma Accelerate's edge → regional pooler split, but simplified: Redis serves the "global-ish low-latency read" role for cached data.

## 9. Deployment & Scaling Considerations

### Initial (v1)
- Single or small number of regions.
- Proxy fleet behind a TCP load balancer (or L4) that supports long-lived connections.
- One Redis Cluster (or managed Redis) in the same region(s).
- Control plane (HTTP API + DB) in the same or adjacent region. Can be the same binary exposing different ports, or a separate deploy.
- Stateless proxies: any instance can handle any tenant's traffic (after fetching decrypted credentials or having access to a secret store).

### Later (global low-latency reads)
- Deploy proxy instances in many PoPs (Kubernetes in multiple clouds, Fly.io, Cloudflare Workers + TCP if possible, or regional VMs).
- Use a **regional Redis** or **global Redis** (Redis Enterprise Active-Active, Dragonfly, or KeyDB with CRDTs) or accept per-region caches (like Prisma Accelerate).
- For cache misses, proxies may need to route the miss to a "pooler" instance that is topologically close to the tenant's MongoDB cluster (to reduce cross-region DB latency on misses). This can be done via consistent hashing on tenantId or explicit regional affinity in tenant config.
- Anycast DNS or smart client-side host selection for the `nance-host` portion of the URI.

**Resilience**:
- Proxies must degrade gracefully: Redis down → passthrough.
- Real Mongo down for a tenant → surface proper errors (don't poison cache).
- Hot reload of tenant policy without restart.
- Blue/green or rolling deploys that don't drop long-lived client connections (draining).

## 10. Observability & Operability

Per-tenant, per-collection, per-operation metrics (Prometheus + Grafana or equivalent):
- Query rate, error rate, p50/p95/p99 latency (cache hit vs miss vs passthrough).
- Cache hit ratio, bytes from cache, evictions, memory usage.
- Backend pool utilization (active/idle connections per tenant).
- Invalidation rate and latency impact.
- Slow query log (sampled, redacted).

Distributed tracing: propagate trace context through to real Mongo where possible (Mongo supports `comment` or custom fields).

Tenant-visible usage: "you saved X queries / Y ms via cache this month" (if billing or quotas involved later).

## 11. Security

- **Credential isolation**: Client apps never see or store real Mongo URIs. Nance tokens can be scoped (read-only tokens, expiring, per-app, etc.).
- **Network**: Nance can sit in a VPC that can reach tenant DBs; tenant DBs can whitelist only Nance's IPs (or use private endpoints + Nance in the right account).
- **Query privacy**: Queries and results transit Nance. Consider field-level redaction or audit sampling. Never log full result sets.
- **Rate limiting & quotas**: Per-tenant QPS, concurrent connections, max cached result size, max concurrent backend connections. Protect noisy neighbors and the real DB.
- **Denial of wallet / cache stampede**: Use request coalescing (single-flight) for the same cache key on miss so 1000 concurrent identical misses only produce 1 real query.
- **mTLS / encryption in transit**: Terminate TLS at the proxy for client connections. Use TLS to real Mongo as specified in the tenant URI.

## 12. Scope, Limitations & Explicit Trade-offs

**Supported in MVP target**:
- Standard CRUD reads (`find`, basic `aggregate`, counts, distinct) with filters, projections, sort, limit, skip.
- Caching decisions based on target collection.
- Writes fully supported (passthrough + invalidation).
- Replica set connections on the backend.
- Most driver features (retries, sessions for txns — with cache bypass, causal consistency — best effort).

**Known hard / deferred**:
- Change streams & tailable cursors: always passthrough (and document that they bypass cache).
- GridFS (large binary files): passthrough.
- Very large result sets: size guard + passthrough recommendation.
- Full replica set topology emulation / secondary reads selection from the client view (we can lie and say "I'm primary" or forward readPreference; drivers may get confused on host lists returned by hello).
- SRV record following for the *real* backend (the driver inside Nance handles it).
- Every exotic command flag and server wire behavior 100% identically (aim for "good enough that real apps and the 4-5 most popular drivers work").

**Consistency trade-off**:
- Cached data can be stale up to TTL (or until write-triggered invalidation).
- Tenants explicitly choose cacheable collections knowing this. High-consistency data (counters, inventory that must be exact) should be marked `enabled: false`.

**Performance trade-off**:
- Cache hit path: extra hop (client→Nance) but served from RAM (Redis) with very low latency. Often net win vs direct DB.
- Cache miss: extra hop + Redis lookup + real query. Under heavy identical read load this is amortized.
- Passthrough: small overhead (proxy cost) but enables pooling.

## 13. Phased Delivery Roadmap (Architecture View)

**Phase 0 — Foundations**
- Control plane: tenant CRUD, encrypted URI storage, basic policy store + API.
- Secrets / KMS integration.
- Basic metrics skeleton.

**Phase 1 — Passthrough Proxy (MVP connectivity)**
- Implement enough Mongo wire protocol to accept connections, auth (SCRAM or token), hello, and forward arbitrary commands to a real pooled MongoClient.
- Validate with real drivers (Node, Python, Java) + mongosh.
- Tenant resolution + isolation.
- No caching yet. This alone already gives pooling value and "change the URI" experience.

**Phase 2 — Read-through Cache**
- Redis integration.
- Cache key generation + normalization for supported read commands.
- Policy engine lookup.
- Store/retrieve + TTL.
- Collection-level invalidation on writes.
- Cache bypass for transactions and non-cacheable ops.
- Size guards and single-flight on miss.

**Phase 3 — Polish & Hardening**
- Better cursor emulation, getMore handling for cached results.
- Per-tenant pool tuning UI.
- Explicit invalidation API + tags (if desired).
- Observability dashboards, slow query analysis, hit-rate alerts.
- Multi-region deployment + cache affinity.
- Client SDK (as a convenience layer on top of the wire protocol, not a requirement).
- Performance testing against realistic workloads.

**Phase 4+**
- Global edge presence.
- Advanced invalidation (predicate-based, dependency tracking).
- Cost attribution / "cache savings" reporting.
- Support for more wire commands and exotic features.

## 14. Open Questions & Decisions to Resolve Before Implementation

1. **Wire protocol depth**: How complete do we need the server-side implementation to be for the first paying/ internal tenants? Which 3-4 drivers are "must work"?
2. **Language/runtime for the data plane**: Go (strong recommendation for systems/proxy work), Rust, or Node with native addons? (Affects hiring and some libs.)
3. **Control plane storage**: Postgres + Prisma/TypeORM, or a lightweight dedicated service? Do we reuse Mongo for config (dogfood)?
4. **Redis topology for global**: Start regional only? Use a hosted global cache product?
5. **Auth mechanism on the wire**: Full SCRAM-SHA-256 (high compat) vs a simpler custom mechanism (easier to implement)?
6. **Default behavior**: When a tenant adds a connection string, are all collections uncached by default (safe) or do we have a "cache everything with 60s" default?
7. **Billing / quotas model** (if this becomes a product): cache hits cheaper? bytes transferred? number of tenants' real connections?
8. **Local development experience**: How does a developer run their app against a real Mongo while using accelerator semantics? (Provide a local nance binary in passthrough-only mode? Or a compose with Redis + a tiny proxy?)
9. **Error semantics**: On cache store failure after a successful miss query — do we surface or swallow? (Usually swallow.)

## 15. Summary

Nance Accelerator gives teams the ability to keep using the MongoDB drivers and ecosystem they already know, while inserting a powerful, tenant-controlled caching + pooling proxy layer with a one-line connection string change.

By making caching **declarative at the collection level** and **read-through by default when enabled**, application code stays clean. The proxy does the smart thing based on central policy.

The architecture deliberately mirrors the successful Prisma Accelerate model (edge-ish cache + regional persistent pooling) but adapted to native MongoDB wire protocol and Redis as the explicit caching store.

This document is the starting point. Next steps after team alignment: pick language, spike the wire protocol acceptance + passthrough (Phase 1), validate with real workloads, then layer on the cache.

---

*End of architecture document. No code changes or implementation performed.*
