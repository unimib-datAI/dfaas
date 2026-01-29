# UV setup and service startup

This project uses the `uv` package manager and a small start script to run the
FastAPI service.

## Prerequisites
- Python 3.12
- `uv` installed (`pipx install uv` or `pip install uv`)

## Python version management
This project pins Python 3.12 via `.python-version`. With the `python-preference`
set to `only-managed`, uv will use a uv-managed Python instead of any system
Python when creating the project environment.

If Python 3.12 is not already managed by uv, it will be downloaded automatically
when you run `uv sync`.

## Install dependencies
```bash
uv sync
```

## Run the service
```bash
./scripts/start.sh
```

### Environment variables
- `HOST` (default: `0.0.0.0`)
- `PORT` (default: `8000`)
- `RELOAD` (set to `1` or `true` to enable autoreload)

Examples:
```bash
HOST=127.0.0.1 PORT=9000 ./scripts/start.sh
RELOAD=1 ./scripts/start.sh
```

## Lockfile
To generate or update the lockfile:
```bash
uv lock
```

## Tests
Install dev dependencies and run tests:
```bash
uv sync --group dev
./scripts/test.sh
```

The test script outputs a terminal coverage summary and writes an HTML report in
`htmlcov/`.

## Lint & format
```bash
./scripts/lint.sh
./scripts/format.sh
```

Inline usage:
```bash
uv run ruff check .
uv run ruff format .
```

## UV cache
If your environment blocks writes to `~/.cache`, set `UV_CACHE_DIR` to a writable
path (the scripts default to `.uv-cache` in the repo).
