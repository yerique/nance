# Phase 4+: Global Scale, Advanced Features & Platform Maturity

**Goal**: Evolve Nance from a solid regional accelerator into a highly available, low-latency global service, while adding the advanced capabilities that become valuable once the core is trusted and widely adopted. This is the "scale, intelligence, and product" phase.

This phase is intentionally open-ended and should be broken into sub-milestones (4.1, 4.2, ...) based on actual demand and measured pain points from Phase 3 usage.

## High-Level Themes

1. **Global / multi-region presence** (the Prisma-Accelerate "edge" dream).
2. **Smarter, lower-staleness invalidation and consistency models**.
3. **Observability, cost, and self-service platform features** (the things that make Nance a real internal product).
4. **Broader wire protocol & topology compatibility** (remove more "known hard" limitations).
5. **Operational scale & resilience** at 10-100x the Phase 3 load.

## Detailed Work Streams

### 1. Global Edge / Multi-PoP Deployment

**Motivation**: For globally distributed applications or users far from the primary MongoDB region, the extra hop to a single regional Nance hurts cache-hit latency. We want cache hits served from a PoP close to the client.

**Key design points** (from architecture + refinements):

- Deploy proxy fleets in multiple regions / PoPs (Kubernetes clusters, Fly.io, Cloudflare + TCP support if viable, bare VMs, etc.).
- **Cache topology choices** (decide per deployment or offer both):
  - **Regional caches** (simpler, like many Prisma setups): Each PoP has its own Redis (or regional Redis). Cache is "close" for that geography. Misses go to the tenant's backend Mongo (which may be far). Staleness is per-region.
  - **Global consistent cache** (harder): Use Redis Enterprise Active-Active, Dragonfly, KeyDB with CRDTs, or a custom replication layer. All proxies see roughly the same cache view. Higher write amplification on invalidations.
- **Miss routing / affinity**:
  - Cache misses (and all writes + passthrough) should preferably be executed against a Nance "pooler" instance that is topologically close to the tenant's real MongoDB cluster. This minimizes cross-region database traffic on misses.
  - Techniques:
    - Consistent hashing on `tenantId` to pick a "home" region for that tenant's backend traffic.
    - Or explicit `preferredPoolerRegion` in the tenant config.
    - Proxies in "far" regions forward miss work (via internal gRPC or a side channel) to the home pooler for that tenant, then cache the result locally (write-through to the local Redis) before replying to the client.
- **Client discovery**:
  - Anycast DNS for the `nance-host` portion of the URI (e.g. `accelerator.nance.dev` resolves to the nearest PoP via anycast).
  - Or smart client-side host selection (less common).
  - SRV records (`mongodb+nance+srv://`) can be provided for the Nance endpoint itself if desired.
- **Challenges to solve**:
  - Invalidation propagation speed across regions (eventual is usually acceptable).
  - "Hot tenant" backend pooling still works – the home pooler for a tenant maintains the long-lived connections to that tenant's Mongo.
  - Observability must be aggregated (central metrics + per-region views).
  - Secrets / KMS access from every PoP (or proxy instances fetch short-lived decrypted config from a central control plane).

**Implementation order suggestion**:
- First: two-region deployment with independent regional Redis (easiest win for latency on cache hits).
- Then: add miss-routing / home pooler affinity.
- Later: experiment with a globally replicated cache store.

**Success signal**: A client in region B sees p99 cache-hit latency close to local Redis latency even when the tenant's MongoDB lives in region A.

### 2. Advanced Invalidation & Consistency Improvements

Beyond Phase 2/3 collection-level flush + TTL:

- **Predicate / query fingerprint invalidation** (very hard in the general case):
  - When caching a read, also record a compact representation of the filter shape (and projection if relevant).
  - On a write, attempt to determine whether the written document would have matched any cached filters for that collection.
  - This is approximate at best for rich Mongo queries (`$or`, `$elemMatch`, `$where`, geospatial, etc.). Expect false positives (over-invalidation) and the occasional false negative (must still rely on TTL).
  - Useful only for high-value collections where you have disciplined query patterns.
- **Explicit tag-based invalidation (full)**:
  - Tenants (or application code via a small helper) can attach tags to cached reads (e.g. `user:42`, `product:789`, `catalog:v3`).
  - Tags are stored alongside cache entries (Redis sets per tag + per entry membership).
  - After important business writes, the app calls the explicit invalidate API with a set of tags.
  - This gives application-level control with stronger consistency for selected data without shortening global TTLs.
- **Versioned snapshots or "cache namespaces"**:
  - Bump a collection-level "cache generation" counter on major writes or via API. All new cache keys incorporate the generation. Old keys naturally expire via TTL. Gives a "flush this entire semantic version" button.
- **Hybrid model documentation**:
  - Official guidance: "Use short TTL + write invalidation for most things. Use tags for high-read, occasionally-written lookup data where you can afford application-level invalidation calls. Never cache data that must be read-after-write consistent inside the same request."

### 3. Cost Attribution, Savings Reporting & Platform Features

Once many teams use Nance, they (and the platform team) want to understand the value:

- **Per-tenant, per-collection "cache economics" metrics** (stored in a time-series DB or aggregated daily):
  - Cache hits vs misses (absolute and %).
  - Queries "saved" (misses that would have gone to Mongo).
  - Bytes served from cache vs from backend.
  - Rough latency saved (using sampled p99 of miss path vs hit path).
  - Estimated connection savings (harder; based on fan-in ratios).
- **Self-service dashboard** (simple web UI or even a Grafana that tenants are given viewer access to):
  - Hit ratio trends.
  - Top cached collections for the tenant.
  - "You avoided ~1.4 million Mongo queries and 420 seconds of DB time this week."
  - Policy editor (nice form over the raw PUT collection policy API).
  - Invalidation button (namespace + tag).
- **Quotas & billing hooks** (if this ever becomes a charge-back or real product):
  - The rate limiting + usage collection from Phase 3 become inputs.
  - Export usage to a billing system or just show internal "cost center" reports.
- **Audit & compliance**:
  - Longer retention of the audit log from control plane.
  - Ability to export "who changed the policy on this collection and when".
  - Sampled query logging with strong redaction for security reviews.

### 4. Broader Wire Protocol & Topology Fidelity

- Support more commands that were previously "passthrough with warning":
  - Full `listDatabases`, `listCollections` with all filters and options.
  - More admin commands that tools expect.
  - Better handling of `explain`, `collStats`, `dbStats`, `currentOp`, etc.
- **Replica set / sharded cluster topology emulation** (if real need appears):
  - Some drivers and tools behave better when the server claims to be part of a replica set.
  - We can return a synthetic `hosts`, `setName`, `setVersion`, `primary`, and a single "member" that is ourselves.
  - Read preference handling: we can honor `secondary` / `nearest` by forwarding with the corresponding read preference to the backend driver, or always send to primary (current simple behavior).
  - This is mostly lying to the client driver in the `hello` / `ismaster` response and in `getMore` / cursor replies. Risky but sometimes necessary.
- **SRV + better discovery**:
  - Provide `mongodb+nance+srv://` records that point at a discovery service or directly at healthy proxy endpoints.
- **Change streams & tailable cursors**:
  - Still almost always passthrough (caching them makes no sense).
  - But we may need to ensure they work reliably through the proxy (long-lived cursors, resume tokens, proper `getMore` chaining on the real change stream cursor).
  - Document clearly that they bypass all caching and go straight to the tenant's Mongo.
- **GridFS, large binary, very large result sets**:
  - Continue to passthrough.
  - Possibly add size-based early rejection or streaming optimizations so the proxy doesn't buffer multi-GB GridFS chunks.

### 5. Operational & Resilience Maturity

- **Global control plane**:
  - Control plane itself may need to become multi-region (active/passive or active/active for reads) with a replicated metadata store (CockroachDB, Aurora Global, or primary Postgres + logical replication).
  - Proxies everywhere need reliable, low-latency access to tenant config + token validation + decrypted backend URIs.
- **Secrets at global scale**:
  - KMS keys may need to be regional or use a global KMS with proper access.
  - Short-lived credential leasing from control plane to proxies (gRPC streaming or pull model) instead of every proxy having direct KMS access.
- **Chaos engineering & continuous validation**:
  - Regular game days: kill regions, kill Redis in one PoP, partition proxies from control plane, massive invalidation storm.
  - Automated "driver matrix" canaries that continuously run the compatibility suite against canary proxies.
- **Capacity planning & auto-scaling**:
  - Better signals for HPA or cluster autoscaler (connection count + QPS + cache memory pressure).
  - Predictive scaling for known traffic patterns.
- **Backup / DR story for Nance itself**:
  - Redis data is ephemeral (it's a cache); losing it is a giant cold-cache event but not data loss.
  - Control plane metadata (tenant list, policies, encrypted URIs) must be backed up and restorable.
  - Document recovery procedures ("how do we bring Nance back if the entire control plane region dies").

### 6. Advanced Client & Integration Features

- **First-class support for more languages** via the wire protocol (already the point) + SDKs where they remove friction.
- **Alternative protocol side-channel** (HTTP or gRPC + BSON) for environments where raw TCP to the nance host is difficult (some serverless, WASM, certain corporate networks).
  - The SDK can transparently use this when the `mongodb+nance://` scheme would be painful.
  - This path can also carry extra metadata (desired cache tags, explicit bypass, request priority).
- **Prisma / ODM integration examples**:
  - Show how to use Nance from Prisma (via the Mongo driver underneath), Mongoose, TypeORM, etc.
  - Possibly a small Prisma extension or preview feature if it adds value.
- **BI / analytics tool compatibility**:
  - Many BI tools speak Mongo wire or use specific commands. Phase 4 work may include targeted support so "MongoDB Compass + BI connector + Metabase" etc. just work when pointed at Nance.

### 7. Future Architecture Experiments (non-committal)

- **Write-through or materialized caching** for specific high-read lookup collections (very different consistency contract).
- **Edge compute** co-located with the proxies (run small JS/Wasm functions that can do post-processing or authorization before returning cached results).
- **Tiered caching** (L1 in-process in the proxy for ultra-hot keys + L2 Redis).
- **Adaptive TTLs** or ML-driven cache admission / eviction policies per collection based on observed hit rate + write rate.
- **Cross-tenant public dataset caching** (if you ever have shared reference data).

## Phasing Within Phase 4 (Suggested Sub-Milestones)

- **4.1 Global two-region + regional Redis** (big latency win, moderate complexity).
- **4.2 Miss routing + home poolers** + improved invalidation propagation.
- **4.3 Cost/savings dashboard + self-service policy UI** + tag invalidation.
- **4.4 Replica set topology lying + expanded command coverage** (if needed by users).
- **4.5 Advanced consistency features** (predicate invalidation experiments, versioned cache namespaces).
- **4.6** "Nance as a platform" features (quotas, chargeback, SLOs with error budget burn alerts, etc.).

Prioritize based on measured user pain (latency complaints from distant regions, desire for stronger consistency on specific collections, need for certain tools to work, etc.).

## Success Signals for "Phase 4 Done" (or a major sub-phase)

- Cache hit latency for a user in Singapore talking to a MongoDB in us-east-1 is within a few ms of what a local Redis in Singapore would deliver.
- A write in any region invalidates the relevant cache entries in all other regions within a small, predictable window (documented).
- Platform team can show a quarterly "cache savings report" to leadership that attributes real infrastructure cost avoidance or performance improvement to Nance.
- All the "must work" drivers + the 3-4 most important internal tools / BI connectors continue to function through Nance with no special configuration (or only the URI change).
- Adding a new PoP is a reasonably scripted and documented process rather than heroic effort.

## Risks Specific to Phase 4+

- **Risk**: Global cache coherence is extremely hard to get right and easy to make worse than regional caches.  
  **Mitigation**: Default to regional caches + explicit opt-in for stronger global consistency on a per-tenant or per-collection basis. Over-communicate the consistency model.
- **Risk**: Complexity explosion. Every new region and advanced feature multiplies the testing surface and failure modes.  
  **Mitigation**: Keep the majority of tenants on simple regional setups. Use feature flags / progressive rollout for fancy invalidation or global cache modes. Invest heavily in automated canaries and contract tests.
- **Risk**: "Platform" features become a distraction from core reliability.  
  **Mitigation**: Only build self-service UI / reporting after the global data plane is stable. The wire protocol + pooling + basic caching is still the killer feature.

## Relationship to the Original Architecture Document

All the "Phase 4+" bullets from section 13 of architecture.md are covered here:
- Global edge presence
- Advanced invalidation (predicate-based, dependency tracking)
- Cost attribution / "cache savings" reporting
- Support for more wire commands and exotic features

Additional items above are natural evolutions discovered through real usage.

## Exit Philosophy

There is no clean "end" to Phase 4+. The service will continue to evolve. The transition out of Phase 3 into this work should feel like:
- "We now trust Nance with the majority of our Mongo traffic."
- "The biggest remaining wins are geographic latency and making the platform self-service and measurable."

When the team starts regularly talking about "the accelerator" the way they talk about "our database" or "our cache layer", you have succeeded.

---
*Phase 4+ is where Nance stops being "a cool proxy we run" and becomes a foundational, globally distributed part of the data platform.*
