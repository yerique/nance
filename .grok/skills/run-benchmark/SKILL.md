---
name: run-benchmark
description: >
  Run Locust MongoDB/proxy load tests in apps/benchmark (cache vs bypass).
  Use when the user wants to benchmark, load test with Locust, compare cache
  performance, or runs /run-benchmark.
---

# /run-benchmark — Locust Nance benchmarks

## Location

`apps/benchmark` — Python 3.11+, Locust + pymongo.

## Setup (once)

```bash
cd apps/benchmark
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
# set MONGO_URI in .env (copy from .env.example)
```

## Seed

```bash
python scripts/seed.py
```

## Recommended runs

**Compare cache vs bypass (one process):**

```bash
locust -f locustfile.py CompareUser --headless -u 50 -r 10 -t 2m \
  --csv=results/compare --html=results/compare.html
```

**Cache only / bypass only:**

```bash
locust -f locustfile.py CacheUser --headless -u 100 -r 20 -t 3m --csv=results/cache
locust -f locustfile.py BypassUser --headless -u 100 -r 20 -t 3m --csv=results/bypass
```

**Mixed (cache reads + real writes):**

```bash
locust -f locustfile.py MixedUser --headless -u 80 -r 15 -t 5m --csv=results/mixed
```

## Rules

- Real collection name in `MONGO_COLLECTION` (never `*_cache`).
- Writes always hit the real collection; cache is read opt-in via `*_cache`.
- Prefer dedicated `loadtest` DB; do not hammer production collections without confirmation.
- PLAIN proxy URIs need a reachable host:port and usually `directConnection=true` (auto when PLAIN detected).

## User classes

| Class | Purpose |
|-------|---------|
| `BypassUser` | `find_bypass` |
| `CacheUser` | `find_cache` |
| `CompareUser` | equal mix of both (best single-run A/B) |
| `MixedUser` | mostly cache reads + inserts |
| `WriteUser` | inserts only |

Full docs: `apps/benchmark/README.md`.
