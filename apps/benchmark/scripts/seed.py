#!/usr/bin/env python3
"""Seed the real MongoDB collection so read benchmarks have data."""

from __future__ import annotations

import sys
import time
from datetime import datetime, timezone
from pathlib import Path

# Allow `python scripts/seed.py` from apps/benchmark
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from nance_benchmark.mongo import build_client, make_payload, real_coll
from nance_benchmark.settings import load_settings


def main() -> None:
    settings = load_settings()
    client = build_client(settings)
    try:
        client.admin.command("ping")
        coll = real_coll(client, settings)
        n = coll.estimated_document_count()
        target = settings.seed_count
        print(f"target={settings.database}.{settings.real_collection} current≈{n} seed_to={target}")
        if n >= target:
            print("already seeded; nothing to do")
            return
        need = target - n
        payload = make_payload(settings.doc_size)
        batch_size = 100
        inserted = 0
        batch = []
        t0 = time.perf_counter()
        for i in range(need):
            batch.append(
                {
                    "lt_seed": True,
                    "lt_tag": "seed",
                    "ts": datetime.now(timezone.utc),
                    "seq": n + i,
                    "payload": payload,
                }
            )
            if len(batch) >= batch_size:
                coll.insert_many(batch, ordered=False)
                inserted += len(batch)
                batch.clear()
                if inserted % 1000 == 0:
                    print(f"  inserted {inserted}/{need}…")
        if batch:
            coll.insert_many(batch, ordered=False)
            inserted += len(batch)
        # helpful index for tag filter reads
        coll.create_index("lt_tag")
        elapsed = time.perf_counter() - t0
        print(f"seeded {inserted} docs in {elapsed:.1f}s")
    finally:
        client.close()


if __name__ == "__main__":
    main()
