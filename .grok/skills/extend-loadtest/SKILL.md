---
name: extend-loadtest
description: >
  Add modes, flags, metrics, or workers to apps/mongo-loadtest following package
  layout and AGENTS.md architecture rules. Use when extending the load tester,
  adding CLI flags, new workload modes, report fields, or runs /extend-loadtest.
---

# /extend-loadtest — Change mongo-loadtest the right way

Implement features in `apps/mongo-loadtest` without breaking package boundaries.

## Read first

- `apps/mongo-loadtest/AGENTS.md` — layout, CLI contract, architecture rules
- `internal/config/config.go` — flags + validation
- `internal/runner/runner.go` — connect, seed, workers, phases
- `internal/stats/stats.go` — samples, phases, breaking point, reports
- `cmd/loadtest/main.go` — entry, signals, report footer

Module: `github.com/taeven/nance/mongo-loadtest` (Go 1.22+). Driver: `go.mongodb.org/mongo-driver`.

## Architecture rules (non-negotiable)

1. **Config only in `internal/config`** — new knobs are flags + optional env, validated in `Load`. Pass `*config.Config` into the runner. Do not parse flags in runner/stats/main beyond `config.Load`.
2. **Workload execution only in `internal/runner`** — workers record via `stats.Collector.Record(Sample)`. Do not compute percentiles in the runner.
3. **Metrics / reports only in `internal/stats`** — phase snapshots, breaking detection, `BuildReport`, `WriteReport`, `RenderMarkdown`, `RedactURI`.
4. **Breaking point** stays application-level on measurement phases (not warmup/seed). Ramp stops when `Collector.Breaking().Detected`.
5. **Ramp** scales `readW = workers`, `writeW = max(1, workers/2)` per step unless the user explicitly wants a new ramp policy (document and implement carefully).
6. **URI credentials** must stay redacted (`stats.RedactURI`). Never log full password URIs in new paths.
7. Prefer dedicated loadtest DB/collection; never default `-drop` on.
8. Connection pool: `MaxPoolSize = max(read+write+50, 100)` — revisit if concurrency model changes.

## Implementation checklist

1. Add/adjust fields in `config.Config` + `flag`/`envOr` + validation in `Load`.
2. Wire config into runner; implement workload or phase behavior.
3. Extend `stats` only if new metrics/report fields are required (JSON tags, MD rendering, tests in `stats_test.go`).
4. Update CLI table in `AGENTS.md` and user-facing bits in `README.md` if behavior is user-visible.
5. Add/adjust unit tests (prefer stats pure logic; runner tests only if practical).
6. Run verification:

```bash
cd apps/mongo-loadtest
go test ./...
go build -o bin/mongo-loadtest ./cmd/loadtest
```

Optional smoke (needs `MONGO_URI`):

```bash
go run ./cmd/loadtest -uri "$MONGO_URI" -mode mixed -duration 15s -warmup 2s
```

Or invoke `/verify-loadtest`.

## Do not invent CLI

Only document flags that exist in `config.Load`. New flags must be implemented before mentioned in skills/docs.

## Common extension patterns

| Ask | Touch |
|---|---|
| New flag / threshold | `config` → runner and/or stats thresholds |
| New op type | `stats.OpKind` + runner workers + report fields |
| New mode | `config` mode switch + runner phase orchestration |
| Richer reports | `Report` / `PhaseSnapshot` + `RenderMarkdown` + tests |
| Pool / connect tweaks | `runner` only, with pool rule above |
