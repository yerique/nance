---
name: run-loadtest
description: >
  Run the Nance mongo-loadtest CLI with safe defaults and correct flags against
  Atlas, self-hosted MongoDB, or the accelerator proxy. Use when the user wants
  to run a load test, hammer MongoDB, measure throughput/latency, or runs
  /run-loadtest.
---

# /run-loadtest — Run mongo-loadtest safely

Execute a load test from `apps/mongo-loadtest` without inventing flags or targeting production app collections.

## Preconditions

1. Work from `apps/mongo-loadtest` (module root).
2. Require a MongoDB URI: user-supplied `-uri` / `MONGO_URI`, or a local default only if they confirm (e.g. `mongodb://127.0.0.1:27017` or proxy `mongodb://demo:<token>@127.0.0.1:27018/...?authMechanism=PLAIN&authSource=$external`).
3. Prefer dedicated DB/collection: `-db loadtest` / `-collection loadtest_docs` (or env `MONGO_DB` / `MONGO_COLLECTION`). Never default to app production collections.
4. Never pass `-drop` unless the user explicitly opts in. Default keeps data (`-keep-data` true).
5. Do not print full URIs with passwords in chat or commit report files that contain secrets (reports already redact URI).

## Steps

1. **Resolve mode** from the user (default `mixed`):
   - `read` — seed then read workers
   - `write` — insert-only
   - `mixed` — concurrent readers + writers (best default)
   - For ramp / breaking point, prefer `/find-breaking-point` instead

2. **Build or run** (either is fine):

```bash
cd apps/mongo-loadtest
go mod tidy
go run ./cmd/loadtest \
  -uri "$MONGO_URI" \
  -db loadtest \
  -collection loadtest_docs \
  -mode mixed \
  -duration 60s \
  -warmup 5s \
  -read-concurrency 100 \
  -write-concurrency 50 \
  -read-batch 10 \
  -write-batch 10 \
  -seed 10000 \
  -doc-size 1024 \
  -output results
```

Or build once and reuse:

```bash
go build -o bin/mongo-loadtest ./cmd/loadtest
./bin/mongo-loadtest -uri "$MONGO_URI" -mode mixed -duration 60s
```

3. **Optional knobs** only if the user asks — use exact flag names from `internal/config/config.go` / AGENTS.md CLI table. Do not invent flags.

4. **Long runs**: use a reasonable shell timeout or run in background; Ctrl+C still writes partial reports when progress exists.

5. **After the run**:
   - Point the user at `results/loadtest-<run-id>.json` and `.md`
   - Summarize peak/avg ops/s, docs/s, p50/p95/p99, error rate, and whether `breaking_point.detected` is true
   - Offer `/analyze-results` for a deeper read of the artifacts

## Proxy / accelerator targets

Phase 1 proxy requires `authMechanism=PLAIN` and username = tenant id, password = raw token. Example shape only (do not log real tokens):

```text
mongodb://demo:<rawToken>@127.0.0.1:27018/loadtest?authMechanism=PLAIN&authSource=$external
```

Compare proxy vs direct Mongo with two runs and `/compare-runs`.

## Safety checklist

- [ ] URI is approved / disposable for extreme concurrency
- [ ] DB/collection are loadtest-dedicated unless user accepts risk
- [ ] `-drop` not set unless explicit
- [ ] Credentials not echoed in full in the reply
- [ ] Atlas connection quotas / disk saturation understood as possible intended signals
