# Phase 1: Passthrough Proxy (MVP Connectivity)

**Goal**: Deliver the core value of "just change the connection string" with connection pooling and tenant isolation, with **zero caching**. A client using `mongodb+nance://...` can talk to Nance exactly like a MongoDB server for standard CRUD. Writes, reads, admin commands, cursors, and transactions all work by forwarding to the tenant's real MongoDB using a pooled backend client.

This phase alone is independently valuable (pooling benefit for serverless/edge functions, centralized auth, single endpoint).

## Objectives & Success Criteria

- Nance proxy accepts TCP connections on a configurable port (default 27017 for familiarity).
- Implements enough of the MongoDB wire protocol (primarily modern OP_MSG) so that popular drivers (Node.js, Python pymongo, Go, Java) + `mongosh` + MongoDB Compass can connect, authenticate, and perform typical operations against a real backend.
- Presents a **single-node replica set member** topology (hello always says `isWritablePrimary: true`, `minWireVersion`/`maxWireVersion` in a reasonable range for Mongo 5+/6+/7+ compatibility).
- Strong tenant isolation: one connection (or request) is permanently bound to exactly one tenant after auth. No cross-tenant leakage possible.
- Real MongoDB connection strings are **never** sent to or known by the client application.
- Full passthrough semantics:
  - All writes go straight through.
  - All reads go straight through (cache logic is Phase 2).
  - Transactions (multi-statement) are supported and never cached (bypass logic will be reused later).
  - Cursors (`find` with batchSize, `getMore`, `killCursors`) are correctly mapped.
- A tenant's backend `MongoClient` (from official driver) is created lazily and pooled inside Nance. Dozens/hundreds of app-side connections collapse to a handful of real connections to the tenant's cluster.
- Validation: end-to-end tests prove real-world apps and tools can use Nance as a drop-in for their existing MongoDB by changing only the URI.
- Basic per-tenant metrics and logging.

## In Scope for Phase 1

- TCP server + wire framing + OP_MSG request/response handling.
- Command document parsing (BSON) sufficient to:
  - Identify the command verb (`find`, `insert`, `aggregate`, `update`, `delete`, `getMore`, `killCursors`, `hello`, `isMaster`, `ping`, `listCollections`, `listIndexes`, `createCollection`, `drop`, `bulkWrite`, etc.).
  - Extract target namespace (`$db` + collection name or command-specific field like `find`, `insert`).
  - Detect transaction context (`lsid` + `txnNumber`).
- Authentication (see "Auth Strategy" below).
- Tenant resolution + per-connection tenant context.
- Backend pool manager (`map[tenantID]*mongo.Client` with proper locking and lazy creation).
- Full command forwarding using the official `mongo-go-driver` high-level APIs (`Database.RunCommand`, collection methods, cursor iteration).
- Proper reply shaping (convert driver results or raw command replies back into valid OP_MSG reply documents with correct cursor IDs, `ok:1`, error shapes the drivers expect).
- Connection lifecycle: client disconnect → cleanup any server-side cursor state.
- Graceful proxy shutdown (stop accepting, drain in-flight, close backend clients cleanly).
- Health/readiness endpoints (HTTP sidecar or on a different port, or admin commands if we expose them).
- Error mapping (Mongo driver errors → proper Mongo wire error replies).

## Out of Scope (Phase 1)

- Any Redis usage or read-through logic.
- Write-triggered or explicit invalidation.
- Sophisticated cursor emulation for cached results (not relevant yet).
- Rate limiting, quotas, circuit breakers (basic resource protection only).
- Multi-region, SRV records for the nance host itself.
- Full replica set / secondary discovery emulation (we lie and say we are primary).
- Change streams / tailable cursors special treatment beyond "passthrough works".
- Client SDKs.

## Wire Protocol Implementation Details

### Minimal Viable Wire Subset (MVP order of implementation)

1. **Framing & transport**
   - Listen on TCP.
   - Read 16-byte MsgHeader (little-endian): messageLength, requestID, responseTo, opCode.
   - Currently only handle `opCode == OP_MSG (2013)`. Log + close or reply "not supported" for legacy OP_QUERY etc. (most modern drivers use OP_MSG exclusively).
   - Support OP_MSG Section Kind 0 (body) + Kind 1 (document sequence for e.g. inserts) where needed.
   - Write replies with correct responseTo = original requestID.

2. **Handshake commands (implement these first – they are what drivers send immediately)**
   - `hello` / `isMaster` / `ismaster` (case variations).
     - Return a synthetic reply:
       ```json
       {
         "isWritablePrimary": true,
         "maxBsonObjectSize": 16777216,
         "maxMessageSizeBytes": 48000000,
         "maxWriteBatchSize": 100000,
         "localTime": "...",
         "connectionId": 1,
         "minWireVersion": 0,
         "maxWireVersion": 21,   // Mongo 7.x range; tune as needed
         "readOnly": false,
         "ok": 1
       }
       ```
     - Do **not** return a `hosts` list or `setName` unless you want to emulate RS more fully (avoid for MVP).
   - `buildInfo`, `getCmdLineOpts`, `ping`, `whatsmyuri`, `getLog`, etc. – many can be stubbed with `ok:1` or minimal realistic data.
   - `listCommands` can be minimal.

3. **Authentication (critical for real drivers)**
   - **Recommended MVP auth strategy for Phase 1**:
     - Support `PLAIN` SASL mechanism (simplest to implement server-side over TLS).
     - Drivers can be instructed with `?authMechanism=PLAIN&authSource=$external` (or admin).
     - On the wire: client sends `saslStart` with mechanism `PLAIN`, payload containing `\0<username>\0<password>`.
     - Server validates:
       - `username` == tenant identifier (stable id from control plane).
       - `password` == the raw API token previously issued by control plane.
     - Use constant-time compare on the token hash or direct lookup.
     - On success reply with `ok:1`, `done: true`.
     - On failure proper Mongo auth error shape (`code: 18`, "Authentication failed").
   - Alternative if PLAIN proves problematic with some drivers: Accept the first command after TCP connect and treat the credentials embedded in the `hello` or a synthetic `$external` auth as the token. Less standard.
   - **SCRAM-SHA-256 full implementation is deferred**. It is complex (nonces, saltedPassword derivation on server side). Document that tenants should use `authMechanism=PLAIN` in their connection string for Phase 1. Add a note in the URI examples.
   - Store validated tenant context on the connection (or request context). All subsequent commands use that tenant.
   - Support re-auth or multiple credentials? Not needed initially; one tenant per TCP connection is fine.

4. **Core database commands**
   - Read commands: `find`, `aggregate`, `count`, `estimatedDocumentCount`, `distinct`, `listCollections`, `listIndexes`.
   - Write commands: `insert`, `update`, `delete`, `findAndModify`, `bulkWrite`, `createCollection`, `drop`, `dropDatabase`, etc.
   - Session / txn commands: `abortTransaction`, `commitTransaction`, `startSession` (mostly passthrough).
   - `getMore`, `killCursors`.
   - Admin / other: anything unknown → attempt passthrough and log at debug level.

5. **Namespace extraction**
   - For `find: "collName"`, the collection is that value; db comes from `$db` field in the command document (or the auth db).
   - Similar for `insert`, `aggregate` (the pipeline target), etc.
   - Special case `admin` db commands (`listDatabases` etc.) – forward them as-is or with restrictions later.

### Forwarding Strategy (using official driver)

- Maintain `sync.Map` or `map[string]*mongo.Client` protected by mutex or per-tenant lazy init with `singleflight`.
- When a tenant context is established (post-auth), obtain or create:
  ```go
  client, err := mongo.Connect(ctx, options.Client().ApplyURI(decryptedRealURI))
  ```
  - Configure pool options from tenant config (or global defaults) in Phase 1: `MaxPoolSize`, `MinPoolSize`, timeouts.
- To execute an arbitrary command:
  - Parse the incoming OP_MSG body BSON into a `bson.D` or `bson.Raw`.
  - Determine target `dbName := cmd["$db"].(string)`.
  - Use `client.Database(dbName).RunCommand(ctx, cmd)` for most things.
  - For collection-specific helpers (`coll.Find`, `coll.InsertMany` etc.) you can map the high-level commands to driver calls. This gives you cursor handling "for free" on the backend side.
- For `getMore`:
  - The driver cursors on the backend side return a `mongo.Cursor`.
  - You must map **client-visible cursorID** (int64 you choose) to the real backend cursor.
  - Store a small in-memory map `map[int64]cursorState` (the state contains the tenant's cursor, last batch, etc.).
  - On `getMore(cursorID, batchSize)` look up and call `cursor.Next` / `cursor.RemainingBatch` or `cursor.All` limited, then shape a proper reply containing `cursor: { id: <same or 0>, nextBatch: [...] }`.
  - `killCursors` cleans the map entry.
- Cursor ID generation: simple atomic counter per proxy process is acceptable (they are opaque to clients and scoped to one Nance connection anyway).
- Large result streaming: when forwarding a find that returns a cursor, do **not** fully drain into memory unless necessary. Use the driver's cursor and stream batches back to the wire client.

### Reply Construction

- Every client OP_MSG gets an OP_MSG reply.
- Body section contains the result document(s) exactly as the backend driver produced them (or a synthetic one for hello).
- For cursors: the `cursor` subdocument must have `id` (int64, 0 means exhausted/no server cursor), `ns`, and `firstBatch` or `nextBatch`.
- Error replies must use the conventional shape: `{ ok: 0, errmsg: "...", code: N, codeName: "..." }`.
- The mongo-go-driver has helpers; you can also round-trip raw BSON when possible to preserve exact field ordering/behavior.

## Tenant Isolation & Security in the Proxy

- After successful auth, the `conn` struct (or a context value) holds `tenantID` immutably.
- Every code path that touches a backend client **must** go through the tenant-scoped getter.
- Backend `mongo.Client` instances are **never** shared across tenants.
- When decrypting the real URI (Phase 0 crypto layer), do it only at the moment of client creation. Hold the decrypted value only in the `mongo.Connect` options; do not store it in logs or long-lived variables.
- On tenant backend rotation (future control plane feature): allow creating a fresh client; old client can be drained over time (close idle connections, let in-flight finish).
- Connection limits: track number of open TCP connections per tenant; have a soft ceiling (return a "too many connections" error shape when exceeded). This protects noisy neighbors.

## Backend Pool Manager (`internal/proxy/pool/` or similar)

- `type PoolManager struct { clients map[string]*mongo.Client; mu sync.Mutex ... }`
- `Get(ctx, tenantID) (*mongo.Client, error)`
- Lazy connect + health check on first use (or background pinger).
- On proxy shutdown: `Disconnect` all clients.
- Consider adding a background goroutine that prunes clients for tenants that have had zero traffic for N minutes (saves resources for very sparse tenants).

## Configuration & Startup for the Proxy

- Same binary layout as Phase 0 or separate image.
- Required env / config:
  - `NANCE_CONTROL_PLANE_URL` or direct DB access? (Recommendation: proxy talks to Postgres directly for token validation + backend URIs, or calls an internal control plane gRPC/HTTP for "resolve tenant + get decrypted config". Start with shared DB access for simplicity; add indirection in Phase 3.)
  - `NANCE_MASTER_KEY` (for decrypting URIs at runtime).
  - Listen address, TLS cert/key if terminating TLS here (recommended: terminate at LB or here).
  - Per-tenant overrides (initially global only).
- The proxy must be able to validate tokens fast. Options (choose one for Phase 1):
  A. Proxy has Postgres read access and does the same token hash lookup as control plane.
  B. Lightweight gRPC "auth service" in control plane; proxy calls it (with short cache).
  Start with **A** (shared DB read path) – simplest. Use read replicas or connection pool tuned for many proxies.

## Detailed Step-by-Step Build Order (highly recommended sequence)

1. **Scaffold proxy command** (empty main that listens on :27017 and accepts TCP).
2. **Wire framing library** (small package that can read/write OP_MSG frames; unit test with hex dumps from real traffic if possible).
3. **Hello / handshake responder** – hard-code a good reply. Use `mongosh` or a Go driver `mongo.Connect` against `mongodb://localhost:27017` (no auth yet) and confirm it doesn't immediately error.
4. **Add minimal auth**:
   - Accept connections with no auth first (great for early iteration and mongosh `--authenticationDatabase admin` tricks).
   - Then implement PLAIN SASL path. Test with Node driver using `authMechanism: 'PLAIN'`.
5. **Namespace + command classification** parser (pure function taking `bson.Raw` → command name + db + coll + isTxn).
6. **Backend pool manager** + first end-to-end passthrough for `ping` and a simple `find` on a known collection (hard-code a tenant + real URI at startup for dev).
7. **Full command router** that dispatches to driver equivalents or `RunCommand`.
8. **Cursor mapping** (`getMore` / `killCursors` support). This usually takes the most debugging.
9. **Error paths & reply shaping** – make sure pymongo, node, etc. see normal `pymongo.errors` or driver errors instead of protocol parse failures.
10. **Tenant resolution from wire credentials** (tie into Phase 0 token table).
11. **Per-connection context + isolation tests**.
12. **Metrics + structured logging** on every command (redact query bodies at info level or sample them).
13. **Graceful lifecycle + tests**.
14. **End-to-end validation matrix** (see below).

## Testing & Validation Strategy (Phase 1 acceptance)

**Must work**:

- `mongosh "mongodb+nance://tenant1:thetoken@127.0.0.1:27017/mydb?authMechanism=PLAIN&authSource=$external"` then `db.coll.find({}).toArray()`, inserts, updates.
- Node.js `MongoClient` with same URI string, standard operations, transactions (`session.withTransaction`).
- Python `pymongo` same.
- Go official driver same.
- A realistic app workload: a small Express or FastAPI service that does reads + writes through Nance while many instances are simulated (prove that Nance's backend pool stays small while client connection count is high).
- Compass can connect and browse (may require some extra hello fields or `listDatabases` support).
- Error cases: bad token → clear auth failure; backend Mongo down → proper "not primary" or network error surfaced to client; unknown command → reasonable error.
- Cursor iteration: large result set returned in multiple `getMore` batches from the client perspective.
- Transaction: begin, read, write, commit; also abort. Verify no partial state.

**Tooling**:
- Integration tests using `testcontainers` (Mongo + Postgres). Spawn proxy in-process or as a sub-process.
- "Driver matrix" script or GitHub workflow matrix that runs small driver snippets against the proxy container.
- Record real wire traffic (with a tcpdump or driver debug) for a few commands to create golden reply tests where helpful.

**Performance smoke**:
- Simple benchmark: 100 concurrent clients each doing 50 small finds. Measure p99 latency vs direct-to-Mongo, and count of actual backend connections opened (should be << 100).

## Local Development Experience

Document a flow like:
```bash
docker compose up -d postgres mongo
# seed a tenant + token via control plane API (or a make seed-proxy target)
go run ./cmd/proxy --config ...
# Then in another terminal or your app:
export MONGO_URI="mongodb+nance://demo_tenant:demo_token@127.0.0.1:27017/mydb?authMechanism=PLAIN&authSource=$external"
node your-app.js   # or python, etc.
```

Provide a `nance` CLI wrapper (or just instructions) so developers don't need to remember the auth params.

## Risks & Mitigations

- **Risk**: Wire protocol is underspecified or drivers are picky about hello fields / cursor shapes / error codes.  
  **Mitigation**: Iterate with real drivers early and often. Keep a "driver compatibility matrix" table in the repo. Start very narrow (Node + mongosh) then expand.
- **Risk**: Cursor mapping bugs cause leaked cursors or wrong data.  
  **Mitigation**: Heavy emphasis on `getMore` tests + `killCursors`. Add timeouts on server-side cursor state.
- **Risk**: Full SCRAM required by some enterprise drivers or tools.  
  **Mitigation**: Document PLAIN requirement clearly. Implement SCRAM only if paying customer or important internal tool demands it in Phase 1.5.
- **Risk**: Performance overhead of proxy even in passthrough is noticeable.  
  **Mitigation**: Measure it. The pooling benefit should outweigh small per-command cost for most serverless use cases. Optimize hot paths (avoid unnecessary BSON marshal/unmarshal where possible by passing `bson.Raw`).

## Open Questions Resolved or Deferred

- Auth mechanism: **PLAIN** for Phase 1 (documented).
- Language: **Go** (already decided).
- Proxy talks directly to Postgres for tenant lookup (simplest).
- Depth of wire: "good enough for CRUD + cursors + txns on the 4 major drivers" is the target. Exotic admin commands and change streams are passthrough attempts; failures are acceptable and logged.

## Deliverables

- Runnable proxy binary that speaks enough wire for real usage.
- End-to-end "change only the URI" success with at least Node, Python, Go drivers + mongosh.
- Clear demonstration that N connection strings from apps result in far fewer connections to the real Mongo.
- All Phase 1 success criteria checked.
- Updated README + runbooks.
- List of unsupported commands observed during testing (for Phase 3 planning).

**Next phase preview**: Phase 2 adds the Redis read-through cache layer **on top of** the working passthrough. Cached collections will short-circuit before the forwarding step for eligible reads.

---
*Phase 1 is the "MVP that already solves the connection pooling problem and gives the Prisma-Accelerate-like drop-in experience without any caching."*
