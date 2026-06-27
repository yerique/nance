# Nance Admin Dashboard

Nuxt admin UI for the [accelerator control plane](../accelerator/README.md). Manage tenants, MongoDB backends (encrypted at rest), cache policies, access tokens, and cache invalidation.

## Features

| Area | What you can do |
|------|-----------------|
| **Tenants** | List, create, open detail view |
| **Connection** | Set encrypted backend MongoDB URI; test connectivity |
| **Cache policy** | Default TTL; per-collection enable/TTL (`db.collection`) |
| **Tokens** | Issue (raw secret once), list, revoke |
| **Invalidate** | Explicit cache invalidation by db/coll/tags |
| **Savings** | Placeholder metrics / PromQL hints |

All browser calls go through **Nuxt server routes** (`/api/*`), which forward to the accelerator with the admin bearer token. The admin token never ships to the client.

## Prerequisites

1. Accelerator control plane running (default `:8080`):
   ```bash
   cd ../accelerator
   make dev-up
   export NANCE_MASTER_KEY="thisisexactly32byteslongforaes256!!"
   make run
   ```
2. Node.js 20+ and npm.

## Setup

```bash
cd apps/admin-dashboard
cp .env.example .env
# edit NANCE_ACCELERATOR_URL / NANCE_ADMIN_TOKEN if needed
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000).

### Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `NANCE_ACCELERATOR_URL` | `http://localhost:8080` | Control plane base URL (server-only) |
| `NANCE_ADMIN_TOKEN` | _(empty)_ | Bearer token if control plane requires auth |

## Scripts

```bash
npm run dev      # dev server :3000
npm run build    # production build
npm run preview  # preview production build
```

## Architecture

```
Browser  →  Nuxt server routes (/api/tenants/…)  →  Accelerator /api/v1/…
                (NANCE_ADMIN_TOKEN injected)
```

API surface mirrors the control plane (see accelerator README):

- `POST/GET /api/v1/tenants`, `GET /api/v1/tenants/{id}`
- `POST …/backend`, `POST …/backend/test`
- `GET …/policy`, `PUT …/policy/defaults`, `PUT …/policy/collections/{db.coll}`
- `POST/GET …/tokens`, `DELETE /api/v1/tokens/{tokenId}`
- `POST …/invalidate`, `GET …/savings`

## Local workflow example

1. Start control plane + infra (`make dev-up`, `make run` in `apps/accelerator`).
2. Start this app (`npm run dev`).
3. Create tenant `demo` in the UI (or use `make seed` in accelerator).
4. Set backend URI (e.g. `mongodb://localhost:27017`).
5. Test connection.
6. Issue a token; copy the raw secret.
7. Enable caching for `mydb.users` under **Cache policy**.
8. Connect clients via the proxy with `authMechanism=PLAIN` (see accelerator README).

## Security notes

- Backend URIs are write-only from the UI; the API never returns them.
- Issued proxy tokens appear only once in the UI — copy immediately.
- Run the dashboard only on trusted networks in production; protect with the same controls as the control plane admin API.