# Phase 3: Polish, Hardening & Production Readiness

**Goal**: Turn the working passthrough + cache system from Phases 0-2 into a reliable, observable, operable, multi-tenant production service. Focus on the rough edges that appear under real load, the needs of platform/SRE teams, and the remaining gaps that prevent broad safe adoption.

Phase 3 is the "make it boring and trustworthy" phase.

## Objectives & Success Criteria

- Cached results can be consumed by drivers that always use cursors / issue `getMore` (emulated short-lived cursors in the proxy for cache hits).
- Strong operational visibility: distributed tracing, slow-query sampling, per-tenant dashboards or at least rich metrics + alerts, audit of cache decisions.
- Resource protection: per-tenant rate limiting (QPS, concurrent connections, max backend connections), circuit breakers on the real Mongo per tenant, memory/CPU bounds inside the proxy.
- Production deployment artifacts: container images, Kubernetes manifests (or equivalent), blue/green or rolling update story that does not drop long-lived client connections, health/readiness that actually reflects ability to serve traffic.
- Explicit invalidation APIs + basic "by tag" support if the team decides it adds value.
- Local development experience is excellent (a `nance` dev mode or compose profile that a single developer can use without a full cluster).
- Performance validated under realistic mixed workloads (cache hit %, write %, different result sizes, many tenants).
- Client convenience SDK (optional but recommended) for languages that want nicer ergonomics or automatic URI building.
- Security hardening: mTLS options, field redaction in logs, token scoping/rotation story, request size limits.
- Clear SLOs / error budget language and the monitoring to back it up.

## Major Work Areas (roughly parallelizable)

### 1. Cursor Emulation for Cached Results

Current limitation from Phase 2 (id:0 full batch) is great for many apps but breaks drivers or code that does:

```js
const cursor = coll.find({...}).batchSize(10);
while (await cursor.hasNext()) { ... }   // may do getMore even for small sets
```

**Implementation**:
- Add an in-memory, per-proxy, short-lived cursor store (protected map or sync.Map).
- On a cache **hit** for a result that the client might page through:
  - Materialize the full docs in memory (they are already small because we applied `maxResultBytes`).
  - Allocate a new server cursorID (atomic int64, namespaced by tenant or globally unique enough).
  - Store: `type cachedCursor { tenantID, docs []bson.Raw, pos int, created time.Time, ns string }`.
  - Reply to the original command with `cursor: { id: <newID>, firstBatch: first N docs according to batchSize or default }`.
- On subsequent `getMore(cursorID, batchSize, ...)`:
  - Lookup the cursor (must belong to the same connection's tenant context or at least the tenant on the wire).
  - Advance `pos`, return next batch or empty + `id:0` when exhausted.
  - Implement `killCursors` to remove entries.
- Add idle timeout + background reaper goroutine (e.g. cursors older than 10 minutes or 5 minutes of inactivity are dropped). This is safe because the data is immutable (snapshot at cache time).
- Memory accounting: bound total bytes held in live cached cursors per tenant or globally. Evict oldest if over limit.
- This state is **soft** — if the proxy restarts, any open cached cursors simply die (clients will get "cursor not found" which is normal Mongo behavior for server restarts too).

Alternative considered: never emulate cursors for cache hits and document "use `.toArray()` or small limits for best cache experience". Emulation is higher value; do it.

### 2. Deeper Observability & Diagnostics

- **Distributed tracing** (OpenTelemetry):
  - Instrument the wire handler entry.
  - Propagate traceparent through to backend Mongo operations where possible (the official driver supports `comment` or `ReadPreference` metadata; we can also add `$comment` containing trace ID on forwarded commands for server-side correlation).
  - Redis spans for cache get/set.
  - Export to OTLP or Jaeger.
- **Slow query / expensive miss log**:
  - Sample (e.g. 1% or all > 200ms) of cache misses and passthrough commands.
  - Log (redacted) command shape, tenant, ns, duration, result size, cache decision.
  - Optionally store recent slow samples in a ring buffer exposed via a debug HTTP endpoint (`/_admin/slow`).
- **Per-tenant usage counters** (in addition to Phase 2 metrics):
  - Queries served from cache vs real (absolute + %).
  - Bytes served from cache.
  - Estimated "Mongo queries avoided" (rough).
  - Active TCP connections, active backend connections, active cached cursors.
- **Cache effectiveness** dashboard (Grafana json or even a static markdown describing the panels). Key graphs: hit ratio over time, latency heatmaps (hit vs miss vs passthrough), invalidation storms, top 10 cached collections by hit count.

### 3. Resource Protection & Fairness (Multi-Tenancy Hardening)

- **Connection limits per tenant**:
  - Track open TCP connections per tenantID in the proxy.
  - Configurable soft + hard max (from tenant record or global defaults + overrides).
  - On new connection after soft limit: log + possibly slow the handshake. Hard limit → immediate Mongo "too many connections" style error reply and close.
- **QPS / command rate limiting**:
  - Token bucket or sliding window per tenant (in-memory with a small goroutine reaper, or backed by Redis for cross-proxy accuracy if you have multiple proxies).
  - Separate buckets for reads vs writes, or one total.
  - When throttled, return a retriable error shape that good drivers will back off on (code 10107 or similar "exceeded rate limit" style).
- **Backend connection pool caps per tenant**:
  - When creating the `mongo.Client` for a tenant, pass `options.Client().SetMaxPoolSize(tenant.MaxBackendConns ?? global)`.
  - This is the ultimate protection for the real MongoDB.
- **Circuit breaker / health per tenant backend**:
  - Simple state machine per tenant: Closed / Open / HalfOpen.
  - Track recent failure rate or consecutive failures on backend commands.
  - In Open state, fail fast with a clear "backend unavailable" error instead of letting every request hang for the driver timeout.
  - Success in HalfOpen → close the breaker.
- **Request size / result size guards** on the wire ingress (protect against huge incoming `insert` batches etc.).

These features are usually configured in the tenant record (control plane) and pushed to the proxy via the policy/config refresh path.

### 4. Explicit Invalidation & Tag Support (Progressive)

- Control plane already has a stub `/invalidate` route from Phase 0.
- In Phase 3 make it real:
  - `POST /api/v1/tenants/{tenant}/invalidate` body or query: `{ "db": "mydb", "coll": "orders" }` or `{ "tags": ["user:123", "catalog"] }`.
  - The control plane can either:
    A. Directly connect to Redis and perform the same invalidation the proxy would (using the known key prefix or registry sets). Simplest if control plane and proxies share Redis.
    B. Publish a Redis Pub/Sub message `nance:invalidate:{tenant}:{db}.{coll}` (or with tags). Every proxy subscribes and runs the local `InvalidateNamespace` logic.
  - Proxy also accepts direct invalidation (authenticated internal call) for advanced use cases.
- Basic tag support (optional but powerful):
  - When caching a read, the tenant policy or a future per-query side channel can declare tags for that entry.
  - Store the cache key also in secondary Redis sets per tag: `nance:tenant:{t}:tag:{tagName}:keys`.
  - On invalidate-by-tag, union the relevant sets and delete.
  - This is more advanced; start with namespace invalidation + explicit full-namespace flush. Add tags only if usage patterns justify.

### 5. Production Deployment & Lifecycle

- **Containerization**:
  - Multi-stage Dockerfiles for tiny images (`gcr.io/distroless/static` or `alpine` with ca-certificates).
  - Separate images or one image with entrypoint switch (`/nance controlplane` vs `/nance proxy`).
- **Kubernetes (or your orchestrator) manifests** (in a `deploy/` folder or separate repo):
  - Deployment for control plane (replicas 2+, PDB).
  - Deployment (or StatefulSet) for proxies (multiple replicas behind a TCP load balancer that supports long-lived connections — important: L4 LB or one that does not aggressively timeout idle conns).
  - Services, headless if needed.
  - ConfigMaps / Secrets (never put master key in ConfigMap; use external-secrets or cloud secret store CSI driver).
  - HorizontalPodAutoscaler on CPU or custom metrics (queries/sec).
  - PodDisruptionBudget.
  - NetworkPolicies (proxy only needs egress to tenant Mongos and Redis; control plane to DB).
- **Graceful connection draining**:
  - On SIGTERM the proxy stops the TCP listener (or marks not-ready), waits for existing connections to close naturally or up to a configurable drain timeout (e.g. 30-120s), then shuts down.
  - During drain: new connections can be rejected with a clear message or redirected if you have multiple LBs.
  - Backend `mongo.Client`s are closed only after all in-flight work drains.
- **Rolling / blue-green**:
  - Because clients hold long-lived TCP connections to Nance, you cannot rely on simple rolling update killing old pods instantly.
  - Strategies:
    - Scale up new version fully, update service / LB endpoints gradually (if your LB supports endpoint draining), then scale down old.
    - Or use a "connection draining" readiness gate.
  - Document the exact procedure for your environment.
- **Health endpoints** (HTTP on a side port or multiplexed):
  - `/healthz` – process alive.
  - `/readyz` – can accept new connections (has DB, can reach Redis optionally, has at least one valid tenant config, etc.).
- **Configuration hot reload**:
  - Proxy should pick up new tenant policies and (in future) rotated backend URIs without restart.
  - Already partially done via refresh loop; make the loop robust and observable ("last policy refresh was 14s ago").

### 6. Local Development & Self-Service Experience (Excellent DX)

- Provide a first-class local accelerator experience so developers do not need a full cloud deployment to use the feature while building.
- Options (implement one primary):
  - A `go run ./cmd/proxy --dev` mode that:
    - Uses a local config file or env to define a handful of "dev tenants".
    - For each dev tenant, the real backend URI points at `mongodb://localhost:27017` or a compose service (the developer runs their own Mongo in Docker).
    - Caching can be enabled with very short or infinite TTLs for local iteration.
    - No KMS – uses the Phase 0 local master key.
  - Or a complete `docker-compose.nance.yml` profile that brings up:
    - nance-controlplane
    - nance-proxy (exposed on 27017)
    - redis
    - a "demo-mongo" instance
    - Seeds one tenant + token + policy pointing demo-mongo as the real backend.
  - Bonus: a tiny CLI `nance` (or just Makefile targets + documented curls) that lets a developer:
    - `nance tenant create localdev`
    - `nance token issue localdev` (prints the connection string ready to copy-paste)
    - `nance policy enable localdev mydb.coll --ttl 30`
- The goal: a new engineer can be "using the accelerator locally" in under 10 minutes after cloning.

### 7. Optional Client SDK (Convenience, Not Requirement)

Because the primary integration is "just change the URI", an SDK is sugar only.

- `@nance/accelerator-client` or language-specific packages (start with one language that your teams use most).
- Accepts either the `mongodb+nance://...` string or explicit `{ endpoint, tenantId, token }`.
- For drivers that make the custom scheme painful, the SDK can construct a normal `mongodb://` pointing at the Nance host but with credentials transformed, or it can speak an alternative HTTP/gRPC protocol to Nance (future escape hatch).
- Also useful for: injecting default options (retry, timeouts), adding Nance-specific metadata, or later providing a clean invalidation helper that calls the control plane.
- Document clearly: "The SDK is optional. The wire protocol path requires zero code changes besides the URI."

### 8. Security Hardening

- Terminate TLS at the proxy (or document that a fronting L4/L7 LB does it). Support providing cert + key or ACME.
- Option for mTLS (client certs) as a second factor or for machine-to-machine in addition to the token in the URI.
- Never log full command documents or result sets at default log levels. Provide a sampling "query redactor" or structured field that is explicitly opt-in for debug.
- Token rotation: control plane can issue new tokens while old ones remain valid for a grace period (store multiple active hashes per tenant or use short-lived JWT-style tokens signed by the control plane; the proxy can verify signature without DB lookup).
- Scope tokens (read-only vs read-write) – the proxy can enforce at command classification time (future).
- Request validation: limit max incoming message size, max batch sizes on inserts, etc.

### 9. Performance Testing & Tuning

- Build or adopt a load generator that replays realistic workloads (mix of cached and uncached collections, varying result sizes, write rate that triggers invalidations).
- Tools: custom Go program using the driver, or `ycsb` adapted for Mongo, or `mongo-perf`.
- Measure and publish:
  - Cache hit path latency p50/p99 (target: < 2-5 ms added vs direct Redis would be).
  - Miss path overhead vs pure Phase 1 passthrough.
  - Connection fan-in ratio (1000 app connections → 8 real backend connections).
  - Behavior under cache stampede (the singleflight should keep it flat).
- Tune: Redis client pipelining, batch invalidation, hot path allocations (sync.Pool for reply buffers?), number of proxy replicas vs CPU.

### 10. Documentation & Runbooks (Critical for Phase 3)

- "Onboarding a new tenant" (control plane steps + how the app team gets the URI).
- "Choosing what to cache" (guidance + anti-patterns: counters, inventory, anything that must be strongly consistent).
- "Understanding staleness" (TTL + write invalidation window).
- "Debugging cache behavior" (how to force a miss, how to inspect keys, how to read metrics).
- "Operating Nance" (scaling proxies, Redis sizing, what to do on Redis outage, backend credential rotation procedure).
- "Driver-specific gotchas" (the compatibility matrix from Phase 1/2, updated with any Phase 3 findings).
- SLO definition example: "99.9% of cacheable reads under 10 ms p99 from Nance when Redis is healthy; 99% of passthrough commands complete within 2x of direct-to-Mongo latency."

## Testing Additions Specific to Phase 3

- Load + chaos tests (run for 30+ minutes under synthetic load while killing Redis, restarting a proxy, doing policy updates).
- Cursor emulation specific tests: open a cursor on a cached result, page through it with multiple getMores, interleave with writes (the cursor should still see the snapshot from cache time), kill it explicitly.
- Multi-proxy scenario (if you can spin two proxy processes/containers): write on one, read on the other – invalidation must be visible (this validates the Redis invalidation path across instances).
- Failure injection on backend per tenant (use a proxy or Toxiproxy in front of the test Mongo) to validate circuit breaker.
- Upgrade/drain simulation in CI if possible.

## Deliverables

- All Phase 3 objectives listed above are demonstrable.
- Production-grade deploy manifests + runbooks that an SRE can follow.
- Local dev compose or `dev` mode that feels first-class.
- Metrics + tracing + sampled slow logs that make it pleasant to operate and debug.
- Rate limiting, connection caps, and breakers protecting tenants from each other and protecting real databases.
- Optional SDK for at least the primary language in use.
- Updated architecture.md or a new `OPERATIONS.md` capturing the reality after this phase.

## Risks & Mitigations

- **Risk**: Connection draining and LB behavior is environment-specific and easy to get wrong, leading to dropped connections or stuck old pods during deploys.  
  **Mitigation**: Document the exact LB + LB health check + terminationGracePeriodSeconds + preStop hook recipe for your primary deployment target. Test the full scale-up-then-scale-down flow manually before declaring Phase 3 done.
- **Risk**: Added features (rate limits, tracing, cursor state) introduce latency or memory leaks in the hot path.  
  **Mitigation**: Measure before/after on the critical path. Make almost everything configurable so it can be dialed to zero cost if needed.
- **Risk**: "Polish" phase keeps growing.  
  **Mitigation**: Ruthless prioritization. Draw a line: anything that is "nice for v2" moves to Phase 4. The exit criteria are explicit.

## Relationship to Earlier Open Questions

- Multi-region + global cache: mostly deferred to Phase 4 (regional Redis + affinity routing can be a Phase 3.5 spike).
- Advanced invalidation (predicate matching on writes): Phase 4.
- Full replica set topology emulation: rarely needed; document why we present as a single primary and when that could be a problem (secondary reads from client, certain driver assumptions).
- Billing/quotas: the rate limit + usage metrics in this phase are the foundation if a product billing layer is added later.

**Exit criteria checklist (high level)**:
- [ ] A driver that does cursor iteration on a 200-document cached result successfully pages with multiple getMores against a cache-emulated cursor and sees a consistent snapshot.
- [ ] Under a 10-tenant load test, no tenant can starve another (rate limits + backend pool caps enforced).
- [ ] A rolling deploy of proxies can be performed without any application seeing connection resets (or with documented acceptable blips).
- [ ] An on-call engineer can answer "why is this collection slow?" using only the dashboards + slow log samples + cache hit ratio panels.
- [ ] A new developer can get a working local `mongodb+nance://...` string for their laptop Mongo in < 10 minutes following the docs.

**Next phase (Phase 4+)**: Global presence (many PoPs + smart routing), advanced invalidation strategies, cost attribution / "you saved X queries this month" reporting, deeper topology emulation, support for more exotic wire commands, and product-oriented features if Nance becomes an internal platform product.

---
*Phase 3 is the difference between "it works in a demo" and "we trust this with production traffic for many teams."*
