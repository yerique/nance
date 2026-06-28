---
name: find-breaking-point
description: >
  Ramp mongo-loadtest concurrency until error rate, success rate, or p99 thresholds
  trip; interpret the breaking_point snapshot. Use when the user wants a breaking
  point, capacity limit, ramp test, "how much can it take", or runs /find-breaking-point.
---

# /find-breaking-point — Ramp until thresholds trip

Find the application-level load level where the deployment (or path to it) fails configured thresholds.

## What "breaking point" means here

It is **not** a guarantee the database crashed. It means the load tester's thresholds were breached on a **measurement** phase (not warmup/seed):

- `error_rate` > `-max-error-rate` (default `0.05`)
- `success_rate` < `-min-success-rate` (default `0.90`)
- successful-op **p99** > `-max-p99` (default `2s`)

Ramp stops when `Collector.Breaking().Detected`. Constant modes can still flag a break in reports without stopping early the same way.

## Steps

1. Confirm target URI and that extreme concurrency is acceptable (Atlas quotas, disk, proxy).
2. Prefer dedicated `loadtest` / `loadtest_docs`. No `-drop` unless explicit.
3. Run **ramp mode** from `apps/mongo-loadtest`:

```bash
cd apps/mongo-loadtest
go run ./cmd/loadtest \
  -uri "$MONGO_URI" \
  -db loadtest \
  -collection loadtest_docs \
  -mode ramp \
  -ramp-start 10 \
  -ramp-step 50 \
  -ramp-max 2000 \
  -ramp-step-duration 15s \
  -max-error-rate 0.05 \
  -max-p99 2s \
  -min-success-rate 0.90 \
  -seed 10000 \
  -output results
```

Ramp worker shape (do not "fix" in docs incorrectly): each step sets `readW = workers`, `writeW = max(1, workers/2)`.

4. Tune only with real flags if the user wants a faster or stricter search:
   - Coarser/faster: larger `-ramp-step`, shorter `-ramp-step-duration`, lower `-ramp-max`
   - Stricter latency: lower `-max-p99` (e.g. `500ms`)
   - Tolerate more errors: raise `-max-error-rate`

5. **Interpret the report** (`results/loadtest-*.json` / `.md`):
   - `breaking_point.detected` — true/false
   - `phase_name`, `read_workers`, `write_workers`
   - `read_ops_per_sec_at_break` / `write_ops_per_sec_at_break`
   - `read_ops_total_at_break` / `write_ops_total_at_break` (cumulative ops at trip)
   - `error_rate_at_break`, `p99_read_at_break`, `p99_write_at_break`, `reason`, `summary`
   - Last healthy ramp phase vs the broken phase (workers and ops/s just before trip)

6. Present a concise verdict, e.g.:

```text
Broke at phase ramp-r520w260: ~X read ops/s (520 readers), ~Y write ops/s (260 writers).
Cumulative at break: ~R reads / ~W writes. Reason: <reason>.
Last non-broken step: ...
```

7. If no break through `-ramp-max`, say so clearly and suggest raising max workers, tightening thresholds, or longer step duration for more stable signal.

## Related

- Baseline constant load: `/run-loadtest`
- Deep dive on files: `/analyze-results`
- Proxy vs direct: two ramps + `/compare-runs`
