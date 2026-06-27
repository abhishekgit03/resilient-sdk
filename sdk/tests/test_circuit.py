import time
import pytest
from resilient.circuit import CircuitBreaker, CircuitBreakerOpen, State


def make_failing(cb):
    @cb.protect
    def fn():
        raise Exception("down")
    return fn


def test_opens_after_threshold():
    cb = CircuitBreaker(failure_threshold=3, recovery_timeout=10)
    fn = make_failing(cb)

    for _ in range(3):
        with pytest.raises(Exception):
            fn()

    assert cb.state == State.OPEN


def test_blocks_when_open():
    cb = CircuitBreaker(failure_threshold=1, recovery_timeout=10)
    fn = make_failing(cb)

    with pytest.raises(Exception):
        fn()

    with pytest.raises(CircuitBreakerOpen):
        fn()


def test_half_open_after_cooldown():
    cb = CircuitBreaker(failure_threshold=1, recovery_timeout=0.1)
    fn = make_failing(cb)

    with pytest.raises(Exception):
        fn()

    time.sleep(0.2)
    assert cb.state == State.HALF_OPEN


def test_closes_on_success():
    cb = CircuitBreaker(failure_threshold=1, recovery_timeout=0.1)

    @cb.protect
    def always_fails():
        raise Exception("down")

    @cb.protect
    def always_succeeds():
        return "ok"

    with pytest.raises(Exception):
        always_fails()

    time.sleep(0.2)
    always_succeeds()
    assert cb.state == State.CLOSED
