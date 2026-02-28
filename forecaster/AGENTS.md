# Agent instructions

## Critical
- Use `uv` exclusively for Python dependency management and execution.
- Do not use `pip`, `pipenv`, or `poetry` in this repo.
- Use the project-managed Python (uv) and respect the pinned version in `.python-version`.

## Quick commands
- Install deps: `uv sync`
- Install dev deps: `uv sync --group dev`
- Run server: `./scripts/start.sh`
- Run tests: `./scripts/test.sh` (runs `uv run pytest`)
- Lint: `./scripts/lint.sh` (or `uv run ruff check .`)
- Format: `./scripts/format.sh` (or `uv run ruff format .`)

## Notes
- The scripts default `UV_CACHE_DIR` to a writable `.uv-cache` in the repo.
- Keep `pyproject.toml` as the single source of dependencies.
- Model reload configuration lives in `docs/reload.md` (manifest + env vars).
- Runtime defaults are in `config/runtime.json` (env overrides are disabled by default).
