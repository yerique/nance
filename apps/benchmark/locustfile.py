"""
Locust load tests for MongoDB and the Nance accelerator proxy.

User classes (pick with -u / web UI, or --class-picker / LOCUST_USER_CLASSES):

  BypassUser   — reads real collection (no cache)
  CacheUser    — reads collection_cache (Nance proxy opt-in cache)
  MixedUser    — mostly cache reads + some writes to real collection
  WriteUser    — insert-only on real collection
  CompareUser  — alternates bypass vs cache reads (side-by-side metrics in Locust UI)

Writes always target the real collection. Reads use either the real name or
``{collection}_cache`` to match the proxy contract.

Environment (see .env.example):

  MONGO_URI, MONGO_DB, MONGO_COLLECTION, DOC_SIZE, MONGO_DIRECT
"""

from __future__ import annotations

import random
import time
from datetime import datetime, timezone

from locust import User, between, events, tag, task
from locust.exception import StopUser

from nance_benchmark.mongo import (
    build_client,
    cache_coll,
    make_payload,
    real_coll,
)
from nance_benchmark.settings import load_settings

# Shared settings loaded once at import (Locust workers each import this module).
try:
    SETTINGS = load_settings()
except SystemExit as exc:
    # Allow `locust --help` without MONGO_URI by deferring hard fail to on_start.
    SETTINGS = None  # type: ignore[assignment]
    _SETTINGS_ERROR = str(exc)
else:
    _SETTINGS_ERROR = ""


def _fire(
    user: User,
    name: str,
    start: float,
    *,
    exception: BaseException | None = None,
    response_length: int = 0,
) -> None:
    """Record a custom (non-HTTP) request in Locust stats."""
    total_ms = (time.perf_counter() - start) * 1000
    user.environment.events.request.fire(
        request_type="MONGO",
        name=name,
        response_time=total_ms,
        response_length=response_length,
        exception=exception,
        context={"user": user.__class__.__name__},
    )


class MongoUser(User):
    """Base user: one MongoClient per simulated user."""

    abstract = True
    wait_time = between(0.01, 0.05)

    def on_start(self) -> None:
        if SETTINGS is None:
            raise StopUser(_SETTINGS_ERROR or "MONGO_URI not configured")
        self.settings = SETTINGS
        self.client = build_client(self.settings)
        try:
            self.client.admin.command("ping")
        except Exception as exc:  # noqa: BLE001 — surface connect errors to Locust
            self.client.close()
            raise StopUser(f"Mongo ping failed: {exc}") from exc
        self.real = real_coll(self.client, self.settings)
        self.cached = cache_coll(self.client, self.settings)
        self._payload = make_payload(self.settings.doc_size)

    def on_stop(self) -> None:
        if getattr(self, "client", None) is not None:
            self.client.close()

    def _read(self, coll, name: str) -> None:
        start = time.perf_counter()
        exc: BaseException | None = None
        length = 0
        try:
            # Mix point-ish and small batch finds (matches prior Go loadtest shape).
            if random.randrange(3) == 0:
                doc = coll.find_one({"seq": random.randrange(1_000_000_007)})
                length = 1 if doc else 0
            else:
                filt = {"lt_tag": {"$exists": True}} if random.randrange(2) == 0 else {}
                cursor = coll.find(filt, {"payload": 0}).limit(10)
                length = sum(1 for _ in cursor)
        except Exception as e:  # noqa: BLE001
            exc = e
        _fire(self, name, start, exception=exc, response_length=length)

    def _write(self, name: str = "insert_real") -> None:
        start = time.perf_counter()
        exc: BaseException | None = None
        batch = [
            {
                "lt_tag": "locust",
                "lt_seed": False,
                "ts": datetime.now(timezone.utc),
                "payload": self._payload,
                "w": i,
            }
            for i in range(10)
        ]
        try:
            self.real.insert_many(batch, ordered=False)
        except Exception as e:  # noqa: BLE001
            exc = e
        _fire(self, name, start, exception=exc, response_length=len(batch))


@tag("bypass", "read")
class BypassUser(MongoUser):
    """Reads only the real collection (cache bypass / direct Mongo)."""

    weight = 1

    @task
    def read_bypass(self) -> None:
        self._read(self.real, "find_bypass")


@tag("cache", "read")
class CacheUser(MongoUser):
    """Reads only ``collection_cache`` (Nance proxy cache path)."""

    weight = 1

    @task
    def read_cache(self) -> None:
        self._read(self.cached, "find_cache")


@tag("mixed")
class MixedUser(MongoUser):
    """Realistic mix: cache reads + real writes (through proxy or direct)."""

    weight = 3

    @task(8)
    def read_cache(self) -> None:
        self._read(self.cached, "find_cache")

    @task(2)
    def write_real(self) -> None:
        self._write()


@tag("write")
class WriteUser(MongoUser):
    """Insert-only against the real collection."""

    weight = 1

    @task
    def write_real(self) -> None:
        self._write()


@tag("compare", "read")
class CompareUser(MongoUser):
    """Equal weight bypass vs cache reads — compare RPS/latency in Locust charts."""

    weight = 2

    @task(1)
    def read_bypass(self) -> None:
        self._read(self.real, "find_bypass")

    @task(1)
    def read_cache(self) -> None:
        self._read(self.cached, "find_cache")


@events.init_command_line_parser.add_listener
def _(parser) -> None:  # type: ignore[no-untyped-def]
    parser.add_argument(
        "--mongo-uri",
        type=str,
        env_var="MONGO_URI",
        default="",
        help="Override MONGO_URI for this run",
        include_in_web_ui=True,
    )


@events.test_start.add_listener
def on_test_start(environment, **_kwargs) -> None:  # type: ignore[no-untyped-def]
    uri = getattr(environment.parsed_options, "mongo_uri", "") or ""
    if uri.strip():
        import os

        os.environ["MONGO_URI"] = uri.strip()
        # Reload module-level settings for this process
        global SETTINGS, _SETTINGS_ERROR
        try:
            SETTINGS = load_settings()
            _SETTINGS_ERROR = ""
        except SystemExit as exc:
            SETTINGS = None
            _SETTINGS_ERROR = str(exc)
