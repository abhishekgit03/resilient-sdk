# resilient-sdk

Adaptive retry SDK + CLI for production Python applications.

Every developer writes ad-hoc retry logic - fixed attempts, guessed backoff values, no observability. **resilient-sdk** replaces that with a zero-config decorator that learns from your failure history and surfaces actionable insights via a CLI.

```python
from resilient import retry

@retry.auto
def call_openai(prompt: str):
    return openai.chat.completions.create(...)
```

```bash
$ resilient report
$ resilient explain openai
$ resilient anomalies
```

---

## How it works

- **`@retry.auto`** - wraps any function (sync or async), classifies exceptions automatically, and applies exponential backoff with jitter
- **Postgres persistence** - every retry event is written to `resilient.events`, multi-pod safe
- **CLI** - queries that data and gives you plain-English reports powered by Gemini

---

## Installation

### Python SDK

```bash
pip install resilient-sdk-core
```

Requires Python 3.10+ and a Postgres database.

### CLI

Download the binary from [GitHub Releases](https://github.com/abhishekgit03/resilient-sdk/releases) or install with Go:

```bash
go install github.com/abhishekgit03/resilient-sdk/cli@latest
```

---

## Setup

### 1. Configure

```bash
resilient init --dsn postgresql://user:pass@host/dbname --gemini-key AIza...
```

This writes `~/.resilient/config.toml`. The SDK reads the same file.

### 2. Use the decorator

```python
from resilient import retry

# Works with any external call - HTTP, DB, queue
@retry.auto
def call_stripe():
    return stripe.PaymentIntent.create(...)

@retry.auto
async def call_openai(prompt: str):
    return await openai.chat.completions.create(...)
```

### 3. Optional - Circuit Breaker

Pair with the circuit breaker to stop retrying a service that's fully down:

```python
from resilient import retry
from resilient.circuit import CircuitBreaker

cb = CircuitBreaker(failure_threshold=5, recovery_timeout=30)

@retry.auto
@cb.protect
def call_openai(prompt: str):
    ...
```

---

## CLI Commands

| Command | Description |
|---|---|
| `resilient init --dsn <dsn> --gemini-key <key>` | One-time setup |
| `resilient report` | Failure summary, last 24h |
| `resilient report --app openai --last 7d` | Scoped report |
| `resilient explain <service>` | AI-powered analysis |
| `resilient explain <service> --last 7d` | Scoped explanation |
| `resilient anomalies` | Services that spiked vs yesterday |
| `resilient top` | Worst offenders in the last hour |

### Example output

```
$ resilient explain openai

Analysing openai (last 7d)...

OpenAI calls are failing at 4.2% over the last 7 days, up from 1.1% the week
before. Failures cluster between 14:00–16:00 UTC. The rate_limit errors suggest
you are retrying inside the same rate-limit window. Recommendation: add a 60s
cooldown after 3 consecutive 429s and consider request batching during peak hours.
```

---

## Error Classification

The SDK classifies exceptions automatically - no configuration needed.

| HTTP Status | Error Type | Strategy |
|---|---|---|
| 429 | `rate_limit` | Exponential backoff + jitter, 5 attempts |
| 500/502/503/504 | `server_error` | Backoff + jitter, 4 attempts |
| 400/401/403/404 | `client_fault` | No retry - fail immediately |
| Timeout exceptions | `transient` | Short jitter, 3 attempts |

Works with any HTTP library (`httpx`, `requests`, `aiohttp`) without importing them.

---

## Database Schema

Auto-created on first run:

```sql
resilient.events  -- one row per retry attempt
resilient.stats   -- aggregated windows (populated by CLI queries)
```

Compatible with any existing Postgres instance. Uses a dedicated `resilient` schema to avoid conflicts.

---

## Tech Stack

| Layer | Technology |
|---|---|
| SDK | Python + Poetry |
| CLI | Go + Cobra |
| Storage | PostgreSQL |
| AI | Gemini 2.5 Flash |

---

## License

MIT
