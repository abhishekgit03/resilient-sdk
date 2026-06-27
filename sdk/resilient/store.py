import asyncio
import logging
from datetime import datetime, timezone
from typing import Optional

import asyncpg

from .config import get_config

logger = logging.getLogger(__name__)

_pool: asyncpg.Pool | None = None
_pool_lock = asyncio.Lock()

_SCHEMA_SQL = """
CREATE SCHEMA IF NOT EXISTS resilient;

CREATE TABLE IF NOT EXISTS resilient.events (
    id          BIGSERIAL PRIMARY KEY,
    service     TEXT        NOT NULL,
    fn          TEXT        NOT NULL,
    attempt     INT         NOT NULL,
    status      TEXT        NOT NULL,   -- 'success' | 'failure'
    error_type  TEXT,                   -- NULL on success
    duration_ms INT         NOT NULL,
    ts          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_events_service_ts
    ON resilient.events (service, ts DESC);

CREATE TABLE IF NOT EXISTS resilient.stats (
    id           BIGSERIAL PRIMARY KEY,
    service      TEXT        NOT NULL,
    time_window  TEXT        NOT NULL,
    failure_rate FLOAT       NOT NULL,
    p95_latency  INT,
    peak_hour    INT,
    computed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
"""


async def _get_pool() -> asyncpg.Pool:
    """Return the shared pool, creating it (and the schema) on first call."""
    global _pool
    if _pool is not None:
        return _pool

    async with _pool_lock:
        if _pool is not None:
            return _pool

        dsn = get_config()["dsn"]
        if not dsn:
            raise RuntimeError(
                "resilient-sdk: no DSN configured. "
                "Set RESILIENT_DSN env var or add 'dsn' to config.toml."
            )
        _pool = await asyncpg.create_pool(dsn=dsn, min_size=2, max_size=10)

        async with _pool.acquire() as conn:
            await conn.execute(_SCHEMA_SQL)

    return _pool


async def record_event(
    service: str,
    fn: str,
    attempt: int,
    status: str,
    error_type: Optional[str],
    duration_ms: int,
) -> None:
    """Write one retry event row. Silently drops on any DB error."""
    try:
        pool = await _get_pool()
        async with pool.acquire() as conn:
            await conn.execute(
                """
                INSERT INTO resilient.events
                    (service, fn, attempt, status, error_type, duration_ms, ts)
                VALUES ($1, $2, $3, $4, $5, $6, $7)
                """,
                service,
                fn,
                attempt,
                status,
                error_type,
                duration_ms,
                datetime.now(timezone.utc),
            )
    except Exception:
        # DB being down must never crash the decorated function
        logger.debug("resilient: failed to record event", exc_info=True)
