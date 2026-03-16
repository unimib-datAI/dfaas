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

### Configuration
Runtime defaults are stored in `config/runtime.json`. Environment variables are
documented below but are not used unless env overrides are explicitly enabled
in code.

### Environment variables
- `HOST` (default: `0.0.0.0`)
- `PORT` (default: `8000`)
- `RELOAD` (set to `1` or `true` to enable autoreload)
- `MODELS_BASE_DIR` (default: `/models/current`)
- `MODELS_TYPE` (required, must match manifest)
- `RELOAD_MODE` (`none` | `poll` | `endpoint`)
- `RELOAD_INTERVAL_SECONDS` (default: `30`)
- `RELOAD_TOKEN` (optional, used for `/reload`)

Examples:
```bash
HOST=127.0.0.1 PORT=9000 ./scripts/start.sh
RELOAD=1 ./scripts/start.sh
```

For model layout and reload details, see `docs/reload.md`.

For local development with the repo layout, set:
```bash
MODELS_BASE_DIR=.
```

Generate a manifest for the current bundle:
```bash
uv run python scripts/generate_manifest.py --models-type regression --base-dir .
```

Or use the shell helper (no Python required):
```bash
MODELS_TYPE=regression ./scripts/generate_manifest.sh
```

Sidecar helper:
```bash
./sidecar/sidecar.sh
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

### E2E tests (apple/container)
E2E tests are included in pytest and require the `container` CLI:
```bash
uv run pytest -m e2e
```

If needed, start the container system first:
```bash
container system start
```

### Full k3s E2E (VM)
This test provisions a VM, installs k3s, deploys the app + sidecar and validates
all endpoints:
```bash
uv run pytest -m e2e_k3s
```

Prerequisites: `limactl` (default) or `multipass`, and either `docker` (default)
or `container` for image builds.

You can tune timeouts with:
- `K3S_BUILD_TIMEOUT_SECONDS` (default 1800)
- `K3S_BUILD_IDLE_SECONDS` (default 300)
- `K3S_SAVE_TIMEOUT_SECONDS` (default 600)
- `K3S_SAVE_IDLE_SECONDS` (default 120)
- `K3S_PREFLIGHT_TIMEOUT_SECONDS` (default 120)
- `K3S_PREFLIGHT_IDLE_SECONDS` (default 30)

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
