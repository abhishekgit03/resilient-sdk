import asyncio
import functools
import inspect
import random
import time
from typing import Any, Callable

from .classifier import RetryStrategy, classify
from .store import record_event


class RetryManager:
    """
    Usage:
        @retry.auto
        def call_openai(...): ...

        @retry.auto
        async def call_stripe(...): ...
    """

    def auto(self, fn: Callable) -> Callable:
        if inspect.iscoroutinefunction(fn):
            return _async_wrapper(fn)
        return _sync_wrapper(fn)


#sync wrapper
def _sync_wrapper(fn: Callable) -> Callable:
    @functools.wraps(fn)
    def inner(*args: Any, **kwargs: Any) -> Any:
        service = fn.__module__.split(".")[0]
        attempt = 0

        while True:
            t0 = time.monotonic()
            try:
                result = fn(*args, **kwargs)
                duration_ms = int((time.monotonic() - t0) * 1000)
                _fire_and_forget(record_event(service, fn.__name__, attempt + 1, "success", None, duration_ms))
                return result

            except Exception as exc:
                duration_ms = int((time.monotonic() - t0) * 1000)
                strategy = classify(exc)
                attempt += 1

                _fire_and_forget(record_event(service, fn.__name__, attempt, "failure", strategy.error_type, duration_ms))

                if not strategy.should_retry or attempt >= strategy.max_attempts:
                    raise

                delay = _backoff(strategy, attempt)
                time.sleep(delay)

    return inner


#async wrapper
def _async_wrapper(fn: Callable) -> Callable:
    @functools.wraps(fn)
    async def inner(*args: Any, **kwargs: Any) -> Any:
        service = fn.__module__.split(".")[0]
        attempt = 0

        while True:
            t0 = time.monotonic()
            try:
                result = await fn(*args, **kwargs)
                duration_ms = int((time.monotonic() - t0) * 1000)
                await record_event(service, fn.__name__, attempt + 1, "success", None, duration_ms)
                return result

            except Exception as exc:
                duration_ms = int((time.monotonic() - t0) * 1000)
                strategy = classify(exc)
                attempt += 1

                await record_event(service, fn.__name__, attempt, "failure", strategy.error_type, duration_ms)

                if not strategy.should_retry or attempt >= strategy.max_attempts:
                    raise

                delay = _backoff(strategy, attempt)
                await asyncio.sleep(delay)

    return inner


def _backoff(strategy: RetryStrategy, attempt: int) -> float:
    """Exponential backoff: base * 2^attempt, plus jitter up to base seconds."""
    delay = strategy.base_delay * (2 ** (attempt - 1))
    if strategy.use_jitter:
        delay += random.uniform(0, strategy.base_delay)
    return delay


import threading

_bg_loop: asyncio.AbstractEventLoop | None = None
_bg_loop_lock = threading.Lock()


def _get_bg_loop() -> asyncio.AbstractEventLoop:
    global _bg_loop
    if _bg_loop is not None:
        return _bg_loop
    with _bg_loop_lock:
        if _bg_loop is not None:
            return _bg_loop
        loop = asyncio.new_event_loop()
        t = threading.Thread(target=loop.run_forever, daemon=True)
        t.start()
        _bg_loop = loop
    return _bg_loop


def _fire_and_forget(coro: Any) -> None:
    """Schedule an async coroutine without blocking the caller."""
    try:
        # Inside an async context — schedule as a task on the running loop
        loop = asyncio.get_running_loop()
        loop.create_task(coro)
    except RuntimeError:
        # Sync context — submit to the persistent background loop
        asyncio.run_coroutine_threadsafe(coro, _get_bg_loop())
