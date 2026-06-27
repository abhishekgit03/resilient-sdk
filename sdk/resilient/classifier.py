from dataclasses import dataclass


@dataclass
class RetryStrategy:
    error_type: str      
    should_retry: bool
    base_delay: float 
    max_attempts: int
    use_jitter: bool


_STRATEGIES: dict[str, RetryStrategy] = {
    "rate_limit": RetryStrategy(
        error_type="rate_limit",
        should_retry=True,
        base_delay=2.0,
        max_attempts=5,
        use_jitter=True,
    ),
    "server_error": RetryStrategy(
        error_type="server_error",
        should_retry=True,
        base_delay=1.0,
        max_attempts=4,
        use_jitter=True,
    ),
    "transient": RetryStrategy(
        error_type="transient",
        should_retry=True,
        base_delay=0.5,
        max_attempts=3,
        use_jitter=True,
    ),
    "client_fault": RetryStrategy(
        error_type="client_fault",
        should_retry=False,
        base_delay=0.0,
        max_attempts=1,
        use_jitter=False,
    ),
    "unknown": RetryStrategy(
        error_type="unknown",
        should_retry=True,
        base_delay=1.0,
        max_attempts=3,
        use_jitter=True,
    ),
}


def classify(exc: Exception) -> RetryStrategy:
    """Map an exception to a retry strategy."""

    status = _extract_status_code(exc)
    if status is not None:
        if status == 429:
            return _STRATEGIES["rate_limit"]
        if status in (500, 502, 503, 504):
            return _STRATEGIES["server_error"]
        if status in (400, 401, 403, 404, 422):
            return _STRATEGIES["client_fault"]

    type_name = type(exc).__name__.lower()
    if any(kw in type_name for kw in ("timeout", "timedout", "timed_out")):
        return _STRATEGIES["transient"]
    if any(kw in type_name for kw in ("connection", "network", "connect")):
        return _STRATEGIES["transient"]

    return _STRATEGIES["unknown"]


def _extract_status_code(exc: Exception) -> int | None:
    """Try common attribute names used by different HTTP libraries."""
    for attr in ("status_code", "status", "code", "response"):
        val = getattr(exc, attr, None)
        if isinstance(val, int):
            return val
        if hasattr(val, "status_code"):
            return val.status_code
    return None
