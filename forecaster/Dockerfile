# Stage 1: Build dependencies
FROM python:3.12-slim-bookworm AS builder

# Install uv
COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

WORKDIR /app
ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy UV_PYTHON=/usr/local/bin/python

# Copy dependency files first for layer caching
COPY pyproject.toml uv.lock ./

# Install dependencies using system Python
RUN uv sync --frozen --no-install-project --no-dev

# Copy application and complete sync (dependency-only; app is not packaged)
COPY . /app
RUN uv sync --frozen --no-install-project --no-dev

# Stage 2: Minimal runtime image (same base for Python compatibility)
FROM python:3.12-slim-bookworm

RUN apt-get update && apt-get install -y libgomp1 && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy virtual environment and application
COPY --from=builder /app/.venv /app/.venv
COPY --from=builder /app /app

ENV PATH="/app/.venv/bin:$PATH"

EXPOSE 8000
CMD ["/app/.venv/bin/python", "-m", "uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
