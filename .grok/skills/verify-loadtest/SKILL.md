---
name: verify-loadtest
description: >
  Build, test, and optionally smoke-run apps/mongo-loadtest. Use when verifying
  the load tester after changes, checking it compiles, running go test, or runs
  /verify-loadtest.
---

# /verify-loadtest — Build, test, optional smoke

Confirm `apps/mongo-loadtest` is healthy after edits or before a serious run.

## Always (no Mongo required)

Run from `apps/mongo-loadtest`:

```bash
cd apps/mongo-loadtest
go mod tidy
go test ./...
go build -o bin/mongo-loadtest ./cmd/loadtest
```

Success criteria:

- All packages pass `go test ./...` (includes `internal/stats` unit tests)
- `bin/mongo-loadtest` (or build output) succeeds with exit 0

If tests fail: fix root cause in the owning package (`config` / `runner` / `stats` / `cmd`) per AGENTS.md boundaries; re-run until green.

## Optional smoke (needs live Mongo)

Only if `MONGO_URI` is set or the user provides a URI. Prefer dedicated loadtest DB/collection. Short duration:

```bash
go run ./cmd/loadtest \
  -uri "$MONGO_URI" \
  -db loadtest \
  -collection loadtest_docs \
  -mode mixed \
  -duration 15s \
  -warmup 2s \
  -read-concurrency 20 \
  -write-concurrency 10 \
  -output results
```

Confirm:

- Process exits 0 (or writes partial report on interrupt)
- `results/loadtest-*.json` and `.md` created
- URI in report is redacted (no plaintext password)
- Footer / verdict printed; breaking_point section present (detected or not)

Do **not** use `-drop` in smoke unless explicitly requested.

## After code changes

If the session modified loadtest code, also:

1. Re-read AGENTS.md rules for any boundary violations in the diff
2. Ensure new flags are validated in `config.Load`
3. Ensure new metrics appear in both JSON and Markdown if user-visible
4. Offer `/analyze-results` on the smoke report if useful

## Report back

Paste concise command outcomes (pass/fail + key error lines). Do not claim smoke success without a URI and a completed run.
