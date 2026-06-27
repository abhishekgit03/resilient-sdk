import threading
import time
from enum import Enum


class State(Enum):
    CLOSED = "closed"       # normal — calls go through
    OPEN = "open"           # tripped — fail immediately
    HALF_OPEN = "half_open" # testing recovery — one call allowed


class CircuitBreakerOpen(Exception):
    """Raised when a call is blocked by an open circuit breaker."""
    def __init__(self, service: str):
        super().__init__(f"Circuit breaker open for '{service}' — service appears to be down")


class CircuitBreaker:
    """
    Usage:
        cb = CircuitBreaker(failure_threshold=5, recovery_timeout=30)

        @retry.auto
        @cb.protect
        def call_openai(...): ...
    """

    def __init__(
        self,
        failure_threshold: int = 5,    # failures before opening
        recovery_timeout: float = 30.0, # seconds to wait before half-open
    ):
        self.failure_threshold = failure_threshold
        self.recovery_timeout = recovery_timeout

        self._state = State.CLOSED
        self._failure_count = 0
        self._opened_at: float | None = None
        self._lock = threading.Lock()

    @property
    def state(self) -> State:
        with self._lock:
            return self._get_state()

    def _get_state(self) -> State:
        if self._state == State.OPEN:
            if time.monotonic() - self._opened_at >= self.recovery_timeout:
                # Cooldown elapsed — move to half-open to test recovery
                self._state = State.HALF_OPEN
        return self._state

    def protect(self, fn):
        """Decorator that wraps a function with circuit breaker protection."""
        import functools

        @functools.wraps(fn)
        def inner(*args, **kwargs):
            with self._lock:
                state = self._get_state()
                if state == State.OPEN:
                    raise CircuitBreakerOpen(fn.__name__)
                if state == State.HALF_OPEN:
                    # Allow through — result determines if we close or re-open
                    pass

            try:
                result = fn(*args, **kwargs)
                self._on_success()
                return result
            except Exception:
                self._on_failure()
                raise

        return inner

    def _on_success(self):
        with self._lock:
            self._failure_count = 0
            self._state = State.CLOSED

    def _on_failure(self):
        with self._lock:
            self._failure_count += 1
            if self._failure_count >= self.failure_threshold:
                self._state = State.OPEN
                self._opened_at = time.monotonic()
