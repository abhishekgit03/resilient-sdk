import os
import sys
from pathlib import Path

if sys.version_info >= (3, 11):
    import tomllib
else:
    import tomli as tomllib


_config: dict | None = None


def get_config() -> dict:
    global _config
    if _config is not None:
        return _config

    config_path = Path(os.getenv("RESILIENT_CONFIG", Path.home() / ".resilient" / "config.toml"))

    if config_path.exists():
        with open(config_path, "rb") as f:
            _config = tomllib.load(f)
    else:
        _config = {}

    if dsn := os.getenv("RESILIENT_DSN"):
        _config["dsn"] = dsn

    return _config
