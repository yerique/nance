---
name: compare-runs
description: >
  Diff two or more mongo-loadtest runs (e.g. accelerator proxy vs direct MongoDB)
  using results JSON/MD. Use when the user wants to compare load tests, A/B
  performance, proxy savings/overhead, or runs /compare-runs.
---

# /compare-runs — Diff load test reports

Compare multiple `results/loadtest-*.json` (and optional `.md`) runs side by side.

## Inputs

1. Identify **2+ run artifacts** — user paths, run IDs, or "latest N" in `apps/mongo-loadtest/results/`.
2. If fewer than two exist, run the missing baselines first (`/run-loadtest` or `/find-breaking-point`) with **aligned** settings where possible:
   - Same `-mode`, duration or ramp profile, concurrency / ramp steps
   - Same `-db` / `-collection` / `-doc-size` / batch sizes
   - Different URI only when comparing targets (proxy vs direct, region A vs B)
3. Prefer identical software revision of `mongo-loadtest` when attributing differences to the target.

## Comparison dimensions

For each run extract:

| Dimension | Fields |
|---|---|
| Identity | `run_id`, `mode`, `mongo_uri_redacted`, db/collection |
| Config | workers, duration, ramp_*, thresholds from `config_summary` |
| Throughput | peak + avg read/write ops/s and docs/s |
| Latency | p50/p95/p99 read & write on comparable measurement phases |
| Reliability | error_rate, success_rate, total errors |
| Capacity | `breaking_point` workers, rates, cumulative ops, reason (or not detected) |
| Verdict | `verdict` string |

## Steps

1. Load each JSON report.
2. Flag **non-comparable** configs (different mode, duration, seed size, doc size) before claiming winners.
3. Build a comparison table (markdown) with one column per run.
4. Compute simple deltas where numeric (B − A and % change for ops/s and p99). Call out direction: higher ops/s is better; lower p99/error_rate is better.
5. For **proxy vs direct**:
   - Note PLAIN auth / proxy port vs backend URI (redacted)
   - Attribute differences carefully: cache hits, extra hop, auth path, connection pooling — do not invent metrics not in the report
6. Conclude with: which run sustained more load, which had better latency/errors, whether breaking points differ meaningfully, and a recommended next experiment.

## Example table skeleton

```markdown
| Metric | Run A (direct) | Run B (proxy) | Δ |
|---|---|---|---|
| Mode | mixed | mixed | — |
| Peak read ops/s | | | |
| Peak write ops/s | | | |
| p99 read | | | |
| p99 write | | | |
| Error rate (worst phase) | | | |
| Break workers (r/w) | | | |
| Break reason | | | |
```

## Safety

Keep URIs redacted. Do not claim statistical significance from single short runs; suggest repeats if the user needs confidence.
