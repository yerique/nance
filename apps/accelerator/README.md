# Nance Accelerator — Control Plane (Phase 0)

This is the **Phase 0** implementation of the Nance Accelerator control plane.

See [../../phase0.md](../../phase0.md) for the full plan and success criteria.

## Quick Start (Local Development)

1. Start infrastructure:
   ```bash
   make dev-up
   ```

2. In another terminal, run the control plane (it will auto-run migrations):
   ```bash
   # Terminal A
   export NANCE_MASTER_KEY="thisisexactly32byteslongforaes256!!"   # 32 bytes (or base64)
   # Optional: export NANCE_ADMIN_TOKEN=supersecret   (if unset, dev allows all requests)
   make run
   # or: go run ./cmd/controlplane
   ```

3. Seed a demo tenant + encrypted backend + policy + token:
   ```bash
   make seed
   ```

   The last step prints a `rawToken`. **Copy it** — it is only shown once.

4. At this point you can:
   - List tenants, policies, etc. via the HTTP API on port 8080.
   - In Phase 1 the proxy will use the issued token with a `mongodb+nance://demo:<rawToken>@...` string.

## Important Environment Variables

- `NANCE_MASTER_KEY` — 32-byte key (raw or base64). **Required** for any backend encryption / test operations.
- `DATABASE_URL` — Postgres connection string.
- `NANCE_ADMIN_TOKEN` — Bearer token for protecting `/api/v1/*` (optional in pure local dev).
- `PORT` — default 8080.

## API Overview (all under /api/v1, protected by Bearer)

- `POST   /tenants` — create tenant `{ "id": "proj_abc", "name": "My Project" }`
- `GET    /tenants/{tenantId}`
- `GET    /tenants`
- `POST   /tenants/{tenantId}/backend` — `{ "uri": "mongodb://real..." }` (stored encrypted)
- `POST   /tenants/{tenantId}/backend/test` — validates connectivity to the real cluster (no URI leaked)
- `GET    /tenants/{tenantId}/policy`
- `PUT    /tenants/{tenantId}/policy/collections/{db.coll}` — set per-collection caching rules
- `PUT    /tenants/{tenantId}/policy/defaults`
- `POST   /tenants/{tenantId}/tokens` — returns `{ "rawToken": "...", "tokenId": "..." }` (copy rawToken)
- `GET    /tenants/{tenantId}/tokens`
- `DELETE /tokens/{tokenId}`

Health: `/healthz`, `/readyz`, `/metrics`

## Security Notes (Phase 0)

- Real MongoDB connection strings are **never** stored in plaintext.
- They are encrypted at rest with AES-256-GCM using the master key.
- The `/backend/test` endpoint proves the URI works without ever returning it.
- Tokens for the data plane are returned in raw form **once**. Only a bcrypt hash is stored.

## Next

Once Phase 0 is solid, we will implement **Phase 1** (the actual MongoDB wire protocol proxy + passthrough using the tenants/tokens/backends created here).

See the sibling `phase*.md` files in the repo root for the full phased plan.
