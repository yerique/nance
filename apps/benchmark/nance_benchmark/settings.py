"""Environment-driven settings for Locust + seed scripts."""

from __future__ import annotations

import os
from dataclasses import dataclass

from dotenv import load_dotenv

load_dotenv()

CACHE_SUFFIX = "_cache"


@dataclass(frozen=True)
class Settings:
    mongo_uri: str
    database: str
    collection: str
    doc_size: int
    seed_count: int
    direct_connection: bool | None  # None = auto from URI

    @property
    def real_collection(self) -> str:
        name = self.collection
        if name.endswith(CACHE_SUFFIX):
            name = name[: -len(CACHE_SUFFIX)]
        return name

    @property
    def cache_collection(self) -> str:
        return self.real_collection + CACHE_SUFFIX


def load_settings() -> Settings:
    uri = os.getenv("MONGO_URI", "").strip()
    if not uri:
        raise SystemExit(
            "MONGO_URI is required (export it or put it in apps/benchmark/.env). "
            "Use a direct Mongo URI or a Nance proxy PLAIN URI from the dashboard."
        )
    direct_env = os.getenv("MONGO_DIRECT", "").strip().lower()
    direct: bool | None
    if direct_env in ("1", "true", "yes", "on"):
        direct = True
    elif direct_env in ("0", "false", "no", "off"):
        direct = False
    else:
        direct = None  # auto

    return Settings(
        mongo_uri=uri,
        database=os.getenv("MONGO_DB", "loadtest").strip() or "loadtest",
        collection=os.getenv("MONGO_COLLECTION", "loadtest_docs").strip() or "loadtest_docs",
        doc_size=max(16, int(os.getenv("DOC_SIZE", "512"))),
        seed_count=max(0, int(os.getenv("SEED_COUNT", "5000"))),
        direct_connection=direct,
    )


def is_nance_proxy_uri(uri: str) -> bool:
    u = uri.lower()
    return (
        "authmechanism=plain" in u
        or "authsource=$external" in u
        or "authsource=%24external" in u
    )
