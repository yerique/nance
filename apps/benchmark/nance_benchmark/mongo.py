"""Shared pymongo client factory for Locust users and seed scripts."""

from __future__ import annotations

from typing import Any

from pymongo import MongoClient
from pymongo.collection import Collection
from pymongo.database import Database

from nance_benchmark.settings import Settings, is_nance_proxy_uri


def build_client(settings: Settings) -> MongoClient:
    kwargs: dict[str, Any] = {
        "serverSelectionTimeoutMS": 15_000,
        "connectTimeoutMS": 10_000,
        "maxPoolSize": 100,
        "minPoolSize": 0,
    }
    use_direct = settings.direct_connection
    if use_direct is None:
        use_direct = is_nance_proxy_uri(settings.mongo_uri)
    if use_direct:
        kwargs["directConnection"] = True
        kwargs["retryWrites"] = False
    # Locust: one MongoClient per simulated user. For the Nance proxy, each client
    # is a tenant TCP connection — keep pool size 1 so high -u does not blow
    # NANCE_PROXY_MAX_CONNS_PER_TENANT (default 200).
    if is_nance_proxy_uri(settings.mongo_uri):
        kwargs["maxPoolSize"] = 1
        kwargs["minPoolSize"] = 0
        kwargs["maxIdleTimeMS"] = 30_000

    return MongoClient(settings.mongo_uri, **kwargs)


def get_db(client: MongoClient, settings: Settings) -> Database:
    return client[settings.database]


def real_coll(client: MongoClient, settings: Settings) -> Collection:
    return get_db(client, settings)[settings.real_collection]


def cache_coll(client: MongoClient, settings: Settings) -> Collection:
    return get_db(client, settings)[settings.cache_collection]


def make_payload(size: int) -> str:
    block = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
    return "".join(block[i % len(block)] for i in range(max(1, size)))
