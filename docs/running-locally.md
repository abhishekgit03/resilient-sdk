# Running resilient-sdk Locally

## One-time setup

```bash
resilient init --dsn postgresql://postgres:postgres@localhost/resilient
```

Or manually create `~/.resilient/config.toml`:
```toml
dsn = "postgresql://postgres:postgres@localhost/resilient"
app_name = "resilient-sdk-dev"
```

---

## Running the CLI

### Option 1 — `go run` (recommended for dev)

No build step needed. Run from the `cli/` directory:

```bash
cd /Users/abhishekdasgupta/projects/Resilient/cli

go run main.go report
go run main.go report --app openai --last 7d
go run main.go top
go run main.go anomalies
go run main.go --help
```

### Option 2 — Build a binary

```bash
cd /Users/abhishekdasgupta/projects/Resilient/cli

go build -o resilient .

./resilient report
./resilient report --app openai --last 7d
./resilient top
./resilient anomalies
```

### Option 3 — Install globally

```bash
cd /Users/abhishekdasgupta/projects/Resilient/cli

go install .

# From any directory:
resilient report
resilient top
resilient anomalies
```

Make sure `~/go/bin` is in your PATH. Add to `~/.zshrc` if needed:
```bash
export PATH="$PATH:$HOME/go/bin"
```

---

## All CLI commands

| Command | Description |
|---|---|
| `resilient init --dsn <dsn>` | Write config to `~/.resilient/config.toml` |
| `resilient report` | Failure summary, last 24h |
| `resilient report --app <name>` | Filter by service name |
| `resilient report --last 7d` | Change time window (1h, 24h, 7d, 30d) |
| `resilient top` | Worst offenders in the last hour |
| `resilient anomalies` | Services whose failure rate spiked vs yesterday |
| `resilient explain <service>` | AI-powered analysis (Week 3) |

---

## Python SDK

Install dependencies (from `sdk/` directory):

```bash
cd /Users/abhishekdasgupta/projects/Resilient/sdk
python3 -m venv .venv
.venv/bin/pip install asyncpg tomli
```

Use in your code:

```python
from resilient import retry

@retry.auto
def call_openai(prompt: str):
    return openai.chat.completions.create(...)

@retry.auto
async def call_stripe():
    ...
```
