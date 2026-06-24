# Phase 0: Foundations & Scaffolding

**Goal**: Establish the project foundation, decide key technologies, set up the control plane skeleton, secrets handling basics, tenant and policy data models, local development environment, and observability hooks. No proxy or caching logic yet.

**Status target**: End of phase produces a runnable "control plane" service that can onboard a tenant, store an (encrypted) real MongoDB URI, store declarative cache policies, and issue simple API tokens. Everything is testable locally via docker-compose + HTTP API or CLI.

## Objectives & Success Criteria

- Clear, documented tech stack decision with rationale.
- Reproducible local dev environment: one `make dev` or `docker compose up` brings up Postgres (control), a test Mongo, and optional Redis.
- Control plane can:
  - Create/list/update/delete tenants.
  - Accept and store a real MongoDB connection string (encrypted at rest).
  - CRUD cache policy documents per tenant (the JSON structure from architecture section 6).
  - Issue/revoke opaque API tokens for use in `mongodb+nance://` URIs.
- Validation endpoint that performs a lightweight "can connect" test using the supplied real URI (ping + listCollections) without storing plaintext.
- Basic audit log of config mutations.
- Prometheus `/metrics` endpoint (even if only process metrics initially).
- All secrets handling decisions made and a dev-friendly envelope encryption path implemented (no plaintext URIs on disk or in logs).
- Project structure, build, test, lint, and basic CI (GitHub Actions) in place under `apps/accelerator/`.
- Local development story documented (how an engineer runs their app against a local nance passthrough later).

## Technology Decisions (Resolve in this phase)

1. **Primary language / runtime for the accelerator**:
   - **Decision**: Go 1.22+ for the entire accelerator (both control plane and data plane).
   - Rationale: Excellent concurrency model for a proxy, mature `mongo-go-driver`, great ecosystem for wire-level work, static binaries, small container images, battle-tested in infrastructure projects (Caddy, Traefik, FerretDB parts, etc.). One language for control + data plane simplifies deployment and credential sharing.
   - Alternative considered: Rust (higher perf ceiling, steeper) or Node (familiar but weaker for long-lived TCP proxy + BSON perf).

2. **Control plane storage**:
   - **Decision**: PostgreSQL (managed or docker in dev).
   - Use `database/sql` + `sqlc` (or `pgx` + `sqlc`) for type-safe queries. Avoid heavy ORM.
   - Why not Mongo for config: Avoids circular dependency for the very thing we're accelerating; Postgres is excellent for relational metadata, audit, and JSONB policies.
   - Schema will live in `migrations/` using a simple tool (golang-migrate or tern).

3. **Secrets / encryption of real Mongo URIs**:
   - Envelope encryption.
   - Dev/local: 32-byte master key from environment variable `NANCE_MASTER_KEY` (base64). Never commit real keys.
   - Prod: Master key lives in AWS KMS / GCP KMS (or Vault transit). On startup (or via a sidecar init), unwrap a per-tenant or global DEK. The DEK is kept in memory only inside the control-plane and proxy processes.
   - Never log the real URI. Provide a `Redact` helper used everywhere.
   - Storage column: `real_mongo_uri_ciphertext bytea`, plus `kms_key_id` or `dek_version` metadata.
   - Rotation path noted but not implemented in Phase 0.

4. **AuthN for control plane API**:
   - Initial: Simple bearer token (admin token) or mutual TLS / network-level protection for internal use.
   - Later (Phase 1+): Proper OIDC/JWT or organization-level auth when this becomes multi-user.
   - For tenant "data plane tokens": Random 32-byte opaque strings. Store `bcrypt` hash (or sha256 + salt) + metadata. Fast path validation can use a Redis-backed allowlist or in-memory map refreshed from control plane.

5. **API framework**:
   - `net/http` + `chi` router (or `gorilla/mux` successor). Lightweight, no magic.
   - OpenAPI spec (or just godoc + example curl) generated or hand-written.
   - Versioned under `/api/v1/`.

6. **Observability**:
   - Prometheus client (`prometheus/client_golang`).
   - Structured logging: `slog` (stdlib) or `zap`. JSON logs.
   - Trace IDs via `context` (OpenTelemetry instrumentation can be added in Phase 3).

7. **Configuration**:
   - `koanf` or simple `envconfig` + a `config.yaml` (optional) for dev.
   - 12-factor: everything important via env.

8. **Testing**:
   - Unit: standard `testing` + `testify` or stdlib.
   - Integration: `testcontainers-go` for Postgres + MongoDB in tests.
   - Later: contract tests between control plane and proxy.

## Repository & Module Layout (create under `apps/accelerator/`)

```
apps/accelerator/
├── cmd/
│   ├── controlplane/
│   │   └── main.go
│   └── proxy/               # (scaffolded in Phase 0, implemented in Phase 1)
│       └── main.go
├── internal/
│   ├── config/
│   ├── controlplane/
│   │   ├── api/
│   │   │   ├── handlers/
│   │   │   ├── middleware/
│   │   │   └── server.go
│   │   ├── auth/
│   │   ├── store/           # postgres queries + models
│   │   └── service/         # tenant service, policy service, token service
│   ├── crypto/              # envelope encryption, redaction helpers
│   ├── model/               # shared DTOs: Tenant, CachePolicy, etc.
│   └── telemetry/
├── migrations/
│   └── *.sql
├── pkg/
│   └── api/                 # public types / client if needed
├── scripts/
│   └── dev/
├── docker-compose.yml
├── Dockerfile.controlplane
├── Dockerfile.proxy         # (Phase 1)
├── Makefile
├── go.mod
├── go.sum
└── README.md                # local dev + architecture link
```

Share as much code as possible between controlplane and future proxy (e.g. `internal/model`, `internal/crypto`, `internal/telemetry`).

## Detailed Implementation Steps (in rough recommended order)

1. **Scaffold the Go module**
   - `cd apps/accelerator && go mod init github.com/yourorg/nance/accelerator` (or appropriate module path).
   - Add common deps in go.mod: `github.com/go-chi/chi/v5`, `github.com/jackc/pgx/v5`, `github.com/prometheus/client_golang`, `github.com/testcontainers/testcontainers-go`, `golang.org/x/crypto` (bcrypt), crypto packages.
   - Set up `.golangci.yml` (or use `gofmt` + `staticcheck` + `errcheck` minimally).
   - Add Makefile targets: `build`, `test`, `lint`, `migrate`, `dev`, `generate`.

2. **Database & Migrations**
   - Write initial migrations:
     - `0001_tenants.up.sql`: id (uuid or text), name, created_at, updated_at, status.
     - `0002_tenant_backends.up.sql`: tenant_id, encrypted_uri, dek_version, created_at, last_validated_at.
     - `0003_cache_policies.up.sql`: tenant_id (pk), default_ttl_seconds, collections jsonb, cache_key_version int, updated_at.
     - `0004_tokens.up.sql`: id, tenant_id, token_hash, description, created_at, expires_at (nullable), revoked_at.
     - `0005_audit_logs.up.sql`: id, tenant_id, actor, action (create_tenant, update_policy, invalidate, ...), payload jsonb, at timestamp.
   - Use `golang-migrate` or embed `tern` or a simple `go run` migrator.
   - Add a `Store` interface + Postgres implementation in `internal/controlplane/store/`.

3. **Crypto / Secrets Layer (`internal/crypto/`)**
   - `Envelope` struct + `Encrypt(plaintext []byte, tenantID string) (ciphertext, nonce, dekID []byte, err)`.
   - `Decrypt(...)`.
   - Dev implementation: AES-256-GCM using a key derived from `NANCE_MASTER_KEY` env (or per-tenant static for simplicity at first).
   - Provide `MustRedactURI(uri string) string` that returns `mongodb://***:***@...` form.
   - Add unit tests with known vectors.
   - Later hook: `KMSClient` interface so prod can swap in real cloud KMS unwrap.

4. **Core Domain Models (`internal/model/`)**
   - `Tenant`, `TenantBackend`, `CachePolicy` (struct with `Collections map[string]CollectionPolicy`).
   - `CollectionPolicy { Enabled bool; TTLSeconds int; MaxResultBytes *int }`.
   - JSON marshaling helpers that match the example in architecture.md section 6.
   - Validation logic (e.g. collection key format `db.coll` or future wildcards).

5. **Control Plane Services**
   - `TenantService`: CreateTenant, GetTenant, ListTenants, UpdateTenantStatus.
   - `BackendService`: SetBackend(tenantID, plaintextURI) → encrypts + stores. `TestConnection(ctx, tenantID)` → temporarily decrypts, creates a short-lived mongo client, runs ping + listCollections, returns redacted success/failure.
   - `PolicyService`: GetPolicy, UpsertCollectionPolicy(tenant, dbColl, policy), SetDefaultTTL, etc. Validates keys.
   - `TokenService`: IssueToken(tenantID, description) → generate cryptographically secure random token (32 bytes → base64url), store bcrypt hash, return the raw token **once**. `ValidateToken(rawToken)` → lookup by prefix or full hash compare, return tenantID or error. `RevokeToken`.
   - All services emit audit events (best effort, never fail the main op).

6. **HTTP API Layer**
   - Routes (chi):
     - `POST   /api/v1/tenants`
     - `GET    /api/v1/tenants/{tenantId}`
     - `GET    /api/v1/tenants`
     - `POST   /api/v1/tenants/{tenantId}/backend`  (body: { "uri": "mongodb://real..." })
     - `POST   /api/v1/tenants/{tenantId}/backend/test`
     - `GET    /api/v1/tenants/{tenantId}/policy`
     - `PUT    /api/v1/tenants/{tenantId}/policy/collections/{dbColl}`  (body: CollectionPolicy)
     - `PUT    /api/v1/tenants/{tenantId}/policy/defaults`
     - `POST   /api/v1/tenants/{tenantId}/tokens` → returns { "token": "sk_live_..." }
     - `DELETE /api/v1/tokens/{tokenId}`
     - `POST   /api/v1/tenants/{tenantId}/invalidate` (for later phases, stub for now)
   - Request/response structs + validation (use `go-playground/validator` or manual).
   - Error responses consistent (`{"error": "...", "code": "..."}`).
   - Admin auth middleware (simple "Authorization: Bearer $NANCE_ADMIN_TOKEN" for Phase 0).
   - Health: `GET /healthz`, `GET /readyz` (checks DB connectivity).

7. **Main Entry (`cmd/controlplane/main.go`)**
   - Parse config.
   - Run migrations on start (or separate command).
   - Initialize store, crypto, services.
   - Start HTTP server on configurable port (default 8080).
   - Graceful shutdown.
   - Wire Prometheus registry + `/metrics`.

8. **Local Development Environment**
   - `docker-compose.yml`:
     - `postgres` (nance-control)
     - `mongo` (for future validation tests, version 7+)
     - `redis` (optional now, will be required Phase 2)
     - (no accelerator containers yet)
   - `scripts/dev/seed.sh` or a `make seed` that creates a demo tenant + posts a real backend (pointing at the compose mongo) + sample policies.
   - Document in root README and in `apps/accelerator/README.md`:
     - How to `make dev-up`
     - How to call the API with curl to onboard yourself
     - How to obtain a token for a future `mongodb+nance://demo:thetoken@127.0.0.1:27017/...`

9. **Observability & Hardening Basics**
   - Prometheus `http_duration_seconds`, `tenant_config_updates_total` counters.
   - Structured logs on every API call (method, path, tenant, latency, status) – redact any URI fields.
   - Panic recovery middleware.
   - Rate limit the control plane admin endpoints lightly (or document that it's internal).

10. **Testing & Validation**
    - Unit tests for crypto, model validation, token hash/compare.
    - Integration test (`internal/controlplane/integration_test.go` or `_test` package):
      - Spin up Postgres + real Mongo via testcontainers.
      - Run migrations.
      - Exercise full CRUD flow for a tenant + set backend + test connection + set policies.
      - Verify that after SetBackend the stored row ciphertext does not contain the original URI bytes.
    - Manual test script: use `mongosh` or a small Go program against the compose mongo to prove the "test connection" path works.
    - Add a `make test-integration` that requires docker.

11. **Documentation & Handoff**
    - Update top-level README with Phase 0 status and link to this file.
    - `ARCHITECTURE.md` (or keep architecture.md) reference.
    - "How to onboard a tenant" runbook in the accelerator README.
    - Explicit call-outs of remaining open questions from architecture section 14 that will be answered in Phase 1 (wire depth, auth mechanism choice).

## Deliverables at End of Phase 0

- Working `controlplane` binary.
- Postgres schema + migrations committed.
- Fully functional tenant + policy + token + backend encryption flow via HTTP.
- `docker compose` + seed data that a new developer can use in < 5 minutes.
- All unit + one solid integration test passing in CI.
- No plaintext real Mongo URIs ever written to logs, stdout, or the DB.
- Decision log (in this phase file or a separate DECISIONS.md) for the tech choices above.

## Risks & Mitigations

- **Risk**: Underestimating crypto / KMS complexity.  
  **Mitigation**: Start with local AES master key + clear interface for KMS. Defer actual cloud KMS wiring until a real deployment target exists.
- **Risk**: Scope creep into proxy work.  
  **Mitigation**: Ruthlessly keep proxy code out of this phase. Proxy `cmd/proxy/main.go` can be a stub that prints "Phase 1 coming".
- **Risk**: Choosing the wrong DB abstraction.  
  **Mitigation**: Use `sqlc` or raw queries behind a narrow `Store` interface. Easy to swap later.

## Open Questions to Close in Phase 0

- Exact module path / org naming.
- Whether control plane and proxy will be one binary with a `--role` flag or separate images (recommend separate images from day one for independent scaling).
- Default cache policy when a tenant is created (all disabled, or a safe 60s default on "common" collections)? Document the choice.
- Initial token format / prefix (`sk_live_`, `nance_`, etc.).

## Exit Criteria Checklist (mark as done when complete)

- [ ] Tech decisions written + reviewed.
- [ ] `go build ./...` + tests green.
- [ ] `docker compose up` + seed + curl create tenant + set backend + test-connection succeeds and shows success without leaking URI.
- [ ] A policy with 3 collections (some enabled, some not) can be stored and retrieved.
- [ ] Token can be issued; the raw token value is never stored in plaintext in the DB (only its hash).
- [ ] Local dev instructions are accurate for a fresh clone.

**Next phase**: Phase 1 builds the data plane proxy that consumes the control plane artifacts (tenants, tokens, backends, policies) to actually accept MongoDB wire connections and forward them.

---
*Phase 0 is purely foundational. No Mongo wire protocol, no Redis, no caching behavior is implemented.*
