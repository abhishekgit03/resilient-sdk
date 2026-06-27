import pytest
from resilient.classifier import classify


class HTTP429(Exception):
    status_code = 429

class HTTP503(Exception):
    status_code = 503

class HTTP403(Exception):
    status_code = 403

class ConnectionTimeout(Exception):
    pass


def test_rate_limit():
    s = classify(HTTP429())
    assert s.error_type == "rate_limit"
    assert s.should_retry is True
    assert s.max_attempts == 5

def test_server_error():
    s = classify(HTTP503())
    assert s.error_type == "server_error"
    assert s.should_retry is True

def test_client_fault_no_retry():
    s = classify(HTTP403())
    assert s.error_type == "client_fault"
    assert s.should_retry is False

def test_timeout_classified_as_transient():
    s = classify(ConnectionTimeout())
    assert s.error_type == "transient"
    assert s.should_retry is True

def test_unknown_exception_retries():
    s = classify(ValueError("something unexpected"))
    assert s.error_type == "unknown"
    assert s.should_retry is True
