# AGENTS.md — mongo-loadtest

Extreme MongoDB read/write throughput load tester for Nance. Point at any MongoDB URI (Atlas, self-hosted, or a Nance accelerator proxy) and measure ops/s, docs/s, latency percentiles, and optional **breaking point** under ramp.

## Layout

```
cmd/loadtest/main.go          # CLI entry, signal handling, report footer
internal/config/config.go     # flags + MONGO_URI / MONGO_DB / MONGO_COLLECTION
internal/runner/runner.go     # connect, seed, workers, constant + ramp phases
internal/stats/stats.go       # samples, phases, breaking point, JSON/MD reports
internal/stats/stats_test.go  # unit tests for stats/redaction/percentiles
results/                      # default output dir (loadtest-<run-id>.{json,md})
bin/mongo-loadtest            # optional built binary
```

Module path: `github.com/taeven/nance/mongo-loadtest` (Go 1.22+). Driver: `go.mongodb.org/mongo-driver`.

## Build & test

Always run from this app directory (`apps/mongo-loadtest`):

```bash
go mod tidy
go test ./...
go build -o bin/mongo-loadtest ./cmd/loadtest
```

Smoke (requires `MONGO_URI`):

```bash
go run ./cmd/loadtest -uri "$MONGO_URI" -mode mixed -duration 15s -warmup 2s
```

## CLI contract (do not invent flags)

| Flag | Env | Default | Notes |
|---|---|---|---|
| `-uri` | `MONGO_URI` | required | Connection string |
| `-db` | `MONGO_DB` | `loadtest` | Prefer dedicated DB |
| `-collection` | `MONGO_COLLECTION` | `loadtest_docs` | Prefer dedicated collection |
| `-mode` | | `mixed` | `read` \| `write` \| `mixed` \| `ramp` |
| `-duration` | | `60s` | Constant modes only |
| `-warmup` | | `5s` | Constant modes |
| `-read-concurrency` / `-write-concurrency` | | 100 / 50 | Worker counts |
| `-read-batch` / `-write-batch` | | 10 / 10 | Docs per op |
| `-seed` | | 10000 | Min docs before read/mixed/ramp |
| `-doc-size` | | 1024 | Payload bytes (min clamped to 16) |
| `-ramp-start` / `-ramp-step` / `-ramp-max` | | 10 / 50 / 2000 | Ramp workers |
| `-ramp-step-duration` | | `15s` | Per ramp step |
| `-max-error-rate` | | `0.05` | Break threshold |
| `-max-p99` | | `2s` | Break on successful-op p99 |
| `-min-success-rate` | | `0.90` | Break threshold |
| `-output` | | `results` | Report directory |
| `-run-id` | | UTC timestamp | Filename suffix |
| `-drop` | | false | Drops collection — opt-in only |
| `-keep-data` | | true | Retain docs after run |

## Architecture rules for code changes

1. **Config only in `internal/config`** — new knobs are flags + optional env, validated in `Load`. Wire through `*config.Config` into runner.
2. **Workload execution only in `internal/runner`** — workers record via `stats.Collector.Record(Sample)`; do not compute percentiles in the runner.
3. **Metrics / reports only in `internal/stats`** — phase snapshots, breaking detection, `BuildReport`, `WriteReport`, `RenderMarkdown`, `RedactURI`.
4. **Breaking point** is application-level: error_rate > max, success_rate < min, or successful-op p99 > max on measurement phases (not warmup/seed). Ramp stops when `Collector.Breaking().Detected`.
5. **Ramp** scales `readW = workers`, `writeW = max(1, workers/2)` per step.
6. **URI credentials** must stay redacted in reports (`stats.RedactURI`). Never log full URIs with passwords in new code paths.
7. Prefer dedicated `loadtest` / `loadtest_docs`; never default `-drop` on. Do not target production app collections unless the user explicitly accepts risk.
8. Connection pool: `MaxPoolSize = max(read+write+50, 100)` — if changing concurrency model, reconsider pool sizing.

## Project skills (`.grok/skills/`)

| Skill | Use when |
|---|---|
| `/run-loadtest` | Run a load test with safe defaults and correct flags |
| `/find-breaking-point` | Ramp until thresholds trip; interpret break snapshot |
| `/analyze-results` | Read `results/loadtest-*.json` / `.md` and summarize |
| `/compare-runs` | Diff two or more runs (e.g. proxy vs direct) |
| `/extend-loadtest` | Add modes, flags, metrics, workers following layout |
| `/verify-loadtest` | Build, test, and optionally smoke the CLI |

## Safety

- Never commit secrets; URIs may appear in shell history / env — keep them out of report files (already redacted) and out of git.
- Extreme concurrency can exhaust Atlas connection quotas and saturate disks — that is often the intended signal, but confirm the target is disposable or approved.
- Interrupt (Ctrl+C) still writes partial reports from `main` when progress exists.
