# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0-or-later license.

from __future__ import annotations

from dataclasses import dataclass
import hashlib
import json
import os
from typing import Any

from model import config_constants
from model.model import Model
from model.runtime_config import RuntimeConfig


@dataclass(frozen=True)
class ModelSet:
    models: dict[str, Model]
    model_version: str
    models_type: str
    manifest: dict[str, Any]

    def get(self, metric: str) -> Model:
        return self.models[metric]


def required_paths(models_type: str) -> list[str]:
    paths = ["scalers/scaler_x/features.joblib"]
    for metric in config_constants.METRICS:
        paths.append(f"scalers/scaler_y/{metric}.joblib")
        paths.append(f"models/{metric}/{models_type}/model.joblib")
    return paths


def load_manifest(path: str) -> dict[str, Any]:
    if not os.path.isfile(path):
        raise FileNotFoundError(f"Manifest file not found: {path}")
    with open(path, "r", encoding="utf-8") as handle:
        manifest = json.load(handle)
    errors = validate_manifest(manifest)
    if errors:
        raise ValueError(f"Invalid manifest: {', '.join(errors)}")
    return manifest


def validate_manifest(manifest: dict[str, Any]) -> list[str]:
    errors: list[str] = []
    if not isinstance(manifest, dict):
        return ["manifest must be a JSON object"]

    required = {
        "model_version": str,
        "models_type": str,
        "created_at": str,
        "sha256": dict,
    }
    for key, expected_type in required.items():
        if key not in manifest:
            errors.append(f"missing '{key}'")
            continue
        if not isinstance(manifest[key], expected_type):
            errors.append(f"'{key}' must be {expected_type.__name__}")

    if "sha256" in manifest:
        sha256_map = manifest.get("sha256", {})
        if isinstance(sha256_map, dict):
            if not sha256_map:
                errors.append("'sha256' must not be empty")
            for rel_path, digest in sha256_map.items():
                if not isinstance(rel_path, str) or not rel_path:
                    errors.append("sha256 keys must be non-empty strings")
                if not isinstance(digest, str) or len(digest) < 10:
                    errors.append(f"sha256 for '{rel_path}' must be a hash string")
        else:
            errors.append("'sha256' must be an object")

    if "sidecar" in manifest:
        sidecar = manifest.get("sidecar")
        if not isinstance(sidecar, dict):
            errors.append("'sidecar' must be an object")
        else:
            poll_interval = sidecar.get("poll_interval_seconds")
            if poll_interval is not None:
                if not isinstance(poll_interval, int) or poll_interval <= 0:
                    errors.append("'sidecar.poll_interval_seconds' must be a positive integer")

    return errors


def compute_sha256(path: str | os.PathLike[str]) -> str:
    digest = hashlib.sha256()
    with open(path, "rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def validate_checksums(base_dir: str, manifest: dict[str, Any]) -> None:
    sha256_map = manifest.get("sha256", {})
    missing: list[str] = []
    mismatched: list[str] = []

    for rel_path, expected in sha256_map.items():
        abs_path = os.path.join(base_dir, rel_path)
        if not os.path.isfile(abs_path):
            missing.append(rel_path)
            continue
        actual = compute_sha256(abs_path)
        if actual != expected:
            mismatched.append(rel_path)

    if missing or mismatched:
        details = []
        if missing:
            details.append(f"missing: {', '.join(missing)}")
        if mismatched:
            details.append(f"mismatched: {', '.join(mismatched)}")
        raise ValueError(f"Checksum validation failed ({'; '.join(details)})")


def get_manifest_fingerprint(path: str) -> str | None:
    if not os.path.isfile(path):
        return None
    return compute_sha256(path)


def build_model_set(config: RuntimeConfig) -> ModelSet:
    manifest = load_manifest(config.manifest_path)
    manifest_models_type = manifest["models_type"]
    if config.models_type and config.models_type != manifest_models_type:
        raise ValueError(
            "MODELS_TYPE does not match manifest models_type "
            f"({config.models_type} != {manifest_models_type})"
        )

    validate_checksums(config.models_base_dir, manifest)

    models_type = config.models_type or manifest_models_type
    models: dict[str, Model] = {}
    for metric in config_constants.METRICS:
        models[metric] = Model(metric, models_type, config.models_base_dir)

    return ModelSet(
        models=models,
        model_version=manifest["model_version"],
        models_type=models_type,
        manifest=manifest,
    )


def build_manifest(
    base_dir: str,
    models_type: str,
    model_version: str,
    created_at: str,
    sidecar_poll_interval_seconds: int | None = None,
) -> dict[str, Any]:
    sha256_map: dict[str, str] = {}
    for rel_path in required_paths(models_type):
        abs_path = os.path.join(base_dir, rel_path)
        if not os.path.isfile(abs_path):
            raise FileNotFoundError(f"Required file not found: {rel_path}")
        sha256_map[rel_path] = compute_sha256(abs_path)

    manifest = {
        "model_version": model_version,
        "models_type": models_type,
        "created_at": created_at,
        "sha256": sha256_map,
    }
    if sidecar_poll_interval_seconds is not None:
        manifest["sidecar"] = {"poll_interval_seconds": sidecar_poll_interval_seconds}
    errors = validate_manifest(manifest)
    if errors:
        raise ValueError(f"Invalid manifest: {', '.join(errors)}")
    return manifest
