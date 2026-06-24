# Multi-region (Phase 4.1)

Deploy independent proxy fleets per PoP, each with:

- `NANCE_REGION` — this PoP id
- `NANCE_KNOWN_REGIONS` — comma-separated all PoP ids (for home hashing)
- Regional Redis (`NANCE_REDIS_ADDR` pointing at local/regional cache)
- Shared control-plane Postgres (or regional read replica for token validation)

Cache hits stay local. Misses and writes should prefer the tenant **home** region
(see `internal/proxy/region`). Explicit invalidation propagates via shared Redis
when using a global/active-active cache; otherwise invalidate per region.
