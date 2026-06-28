# Nance Admin Dashboard

Nuxt admin UI for the [accelerator control plane](../accelerator/README.md). Email OTP login, organizations (tenants), members/invites, MongoDB backends, cache TTL overrides, proxy tokens, and invalidation.

## Features

| Area | What you can do |
|------|-----------------|
| **Auth** | Email + verification code; name collected on onboarding |
| **Organizations** | List yours, create, accept invites |
| **Members** | Invite by email, roles, revoke invites |
| **Connection** | Set encrypted backend MongoDB URI; test connectivity |
| **Caching** | Default **60s** TTL for all `*_cache` queries; optional per-collection TTL overrides |
| **Tokens** | Issue (raw secret once), list, revoke |
| **Invalidate** | Explicit cache flush by real db/collection/tags |
| **Savings** | Placeholder metrics / PromQL hints |

### How caching works (important)

- Clients **opt in per query** with a `_cache` suffix: `db.orders_cache.find(...)` → real collection `orders` + Redis (default **60 seconds**).
- `db.orders.find(...)` always hits MongoDB (no cache).
- This UI does **not** enable/disable caching per collection. It only sets the **default TTL** and optional **overrides** for real collection names (`mydb.orders`, not `mydb.orders_cache`).
- Writes to the real collection invalidate that namespace.

Browser calls go through **Nuxt server routes** (`/api/*`), which forward the user session bearer (or `NANCE_ADMIN_TOKEN` fallback) to the control plane.

## Prerequisites

1. Accelerator control plane running (default `:8080`).
2. Node.js 20+ and npm.

## Setup

```bash
cd apps/admin-dashboard
cp .env.example .env   # if present
# NANCE_ACCELERATOR_URL / NANCE_ADMIN_TOKEN as needed
npm install
npm run dev
```

Open [http://localhost:3000](http://localhost:3000) → sign in with email (code is logged by the control plane in dev).

### Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `NANCE_ACCELERATOR_URL` | `http://localhost:8080` | Control plane base URL (server-only) |
| `NANCE_ADMIN_TOKEN` | _(empty)_ | Optional platform admin bearer (server-only fallback) |

## Local workflow example

1. Start control plane + Redis (if using cache).
2. `npm run dev` in this app; sign in.
3. Create an organization; set backend URI; test connection.
4. Under **Caching**, confirm default TTL (60s) or set overrides.
5. Issue a proxy token; connect clients with `authMechanism=PLAIN`.
6. In the app, use `collection_cache` for cached reads and plain `collection` to bypass.

## Security notes

- Backend URIs are write-only from the UI.
- Issued proxy tokens appear only once — copy immediately.
- Prefer user sessions over a long-lived admin token in production.
