---
name: analyze-results
description: >
  Read mongo-loadtest results under apps/mongo-loadtest/results (loadtest-*.json /
  .md), summarize throughput, latency percentiles, phases, verdict, and breaking
  point. Use when the user wants to analyze load test output, interpret a report,
  explain stats, or runs /analyze-results.
---

# /analyze-results — Summarize load test artifacts

Turn `results/loadtest-<run-id>.{json,md}` into a clear performance narrative.

## Locate artifacts

Default directory: `apps/mongo-loadtest/results/`.

```bash
ls -lt apps/mongo-loadtest/results/loadtest-*.{json,md} 2>/dev/null | head -20
```

Prefer the newest pair unless the user names a `run_id` or path. Prefer **JSON** for precise fields; use **Markdown** for human tables already rendered.

## Report schema (key fields)

From `internal/stats.Report`:

| Field | Use for |
|---|---|
| `run_id`, `generated_at` | Identity |
| `mongo_uri_redacted`, `database`, `collection`, `mode` | Target context |
| `config_summary` | Concurrency, duration, thresholds |
| `phases[]` | Per-phase ops, docs/s, latency, error/success, workers, `broken` |
| `totals` | Aggregate ops/docs and avg rates across measurement phases |
| `throughput_peak` | Peak rates |
| `breaking_point` | Threshold trip snapshot |
| `verdict` | High-level outcome string |

Phase kinds: `warmup`, `seed`, `read`, `write`, `mixed`, `ramp`. Ignore warmup/seed when judging sustained capacity unless seeding itself failed.

Latency fields in JSON are often `*_ns` durations; also show human strings from MD (`p50`/`p95`/`p99`).

## Analysis steps

1. Open the chosen JSON (and MD if helpful). Confirm mode and redacted URI.
2. List measurement phases chronologically with: workers, duration, read/write ops/s, docs/s, error_rate, success_rate, p50/p95/p99 for read and write.
3. Call out **peak** vs **average** throughput from `throughput_peak` and `totals`.
4. State **breaking_point**: detected or not; if yes, phase, workers, rates, cumulative ops, reason.
5. Note anomalies: single phase with huge error spike, p99 cliff while ops/s still high, ramp steps that never stabilized (step duration too short).
6. Give a short **recommendation**: raise concurrency, tune proxy/cache policy, re-run ramp with different thresholds, or compare another target.

## Output format (use in the reply)

```markdown
## Run <run_id>
- Target: <uri redacted> / <db>.<collection>
- Mode: <mode> | Config highlights: ...

## Throughput
- Peak / avg read ops/s, write ops/s, docs/s

## Latency (measurement phases)
- Table or bullets: p50 / p95 / p99 read & write

## Errors
- Overall and worst phase error_rate / success_rate

## Breaking point
- Detected? details or "not detected through ramp-max / duration"

## Verdict
- One paragraph + next experiment suggestion
```

## Safety

Do not re-expand redacted credentials. Do not commit large result dumps unless the user asks.
