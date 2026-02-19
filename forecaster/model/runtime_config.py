# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0-or-later license.

from __future__ import annotations

from dataclasses import dataclass
import json
import os
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class RuntimeConfig:
    models_base_dir: str
    models_type: str | None
    manifest_filename: str
    reload_mode: str
    reload_interval_seconds: int
    reload_token: str | None

    @property
    def manifest_path(self) -> str:
        return os.path.join(self.models_base_dir, self.manifest_filename)


DEFAULT_CONFIG_PATH = Path(__file__).resolve().parents[1] / "config" / "runtime.json"


def _load_file(path: Path) -> dict[str, Any]:
    if not path.is_file():
        raise FileNotFoundError(f"Runtime config not found: {path}")
    return json.loads(path.read_text(encoding="utf-8"))


def _apply_env_overrides(data: dict[str, Any]) -> dict[str, Any]:
    overrides = {
        "models_base_dir": os.getenv("MODELS_BASE_DIR"),
        "models_type": os.getenv("MODELS_TYPE"),
        "manifest_filename": os.getenv("MODELS_MANIFEST"),
        "reload_mode": os.getenv("RELOAD_MODE"),
        "reload_interval_seconds": os.getenv("RELOAD_INTERVAL_SECONDS"),
        "reload_token": os.getenv("RELOAD_TOKEN"),
    }
    for key, value in overrides.items():
        if value is not None:
            data[key] = value
    return data


def from_config(path: Path | None = None, allow_env_overrides: bool = False) -> RuntimeConfig:
    config_path = path or Path(os.getenv("RUNTIME_CONFIG_PATH", DEFAULT_CONFIG_PATH))
    data = _load_file(config_path)
    if allow_env_overrides:
        data = _apply_env_overrides(data)

    return RuntimeConfig(
        models_base_dir=data.get("models_base_dir", "."),
        models_type=data.get("models_type"),
        manifest_filename=data.get("manifest_filename", "manifest.json"),
        reload_mode=data.get("reload_mode", "none"),
        reload_interval_seconds=int(data.get("reload_interval_seconds", 30)),
        reload_token=data.get("reload_token"),
    )
