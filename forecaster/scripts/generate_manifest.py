# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0-or-later license.

from __future__ import annotations

import argparse
import json
import os
import sys
from datetime import datetime, timezone
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
if str(REPO_ROOT) not in sys.path:
    sys.path.insert(0, str(REPO_ROOT))

from model.model_store import build_manifest


def _default_version() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H%M%SZ")


def _default_created_at() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate manifest.json for the current model bundle."
    )
    parser.add_argument(
        "--base-dir",
        default=os.getenv("MODELS_BASE_DIR", "."),
        help="Base directory that contains models/ and scalers/.",
    )
    parser.add_argument(
        "--models-type",
        default=os.getenv("MODELS_TYPE"),
        help="Model type (e.g. regression, quantile005, quantile095).",
    )
    parser.add_argument(
        "--model-version",
        default=os.getenv("MODEL_VERSION", _default_version()),
        help="Version string to store in the manifest.",
    )
    parser.add_argument(
        "--created-at",
        default=os.getenv("MODEL_CREATED_AT", _default_created_at()),
        help="Created-at timestamp for the manifest.",
    )
    parser.add_argument(
        "--output",
        default="manifest.json",
        help="Output filename (relative to base dir unless absolute).",
    )
    parser.add_argument(
        "--sidecar-poll-interval",
        default=os.getenv("SIDECAR_POLL_INTERVAL_SECONDS"),
        help="Sidecar poll interval seconds to embed in manifest.",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print manifest to stdout without writing a file.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    base_dir = Path(args.base_dir).resolve()
    models_type = args.models_type
    config_data = None
    if not models_type:
        config_path = base_dir / "config" / "runtime.json"
        if config_path.is_file():
            config_data = json.loads(config_path.read_text(encoding="utf-8"))
            models_type = config_data.get("models_type")

    if not models_type:
        raise SystemExit("MODELS_TYPE is required (env, config, or --models-type).")
    sidecar_interval = args.sidecar_poll_interval
    if sidecar_interval is None and config_data:
        sidecar_interval = config_data.get("sidecar_poll_interval_seconds")
    if isinstance(sidecar_interval, str) and sidecar_interval.strip():
        try:
            sidecar_interval = int(sidecar_interval)
        except ValueError as exc:
            raise SystemExit("Invalid sidecar poll interval.") from exc
    if not isinstance(sidecar_interval, int):
        sidecar_interval = None

    manifest = build_manifest(
        str(base_dir),
        models_type,
        args.model_version,
        args.created_at,
        sidecar_poll_interval_seconds=sidecar_interval,
    )
    payload = json.dumps(manifest, indent=2, sort_keys=True)

    if args.dry_run:
        print(payload)
        return 0

    output_path = Path(args.output)
    if not output_path.is_absolute():
        output_path = base_dir / output_path
    output_path.write_text(payload + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
