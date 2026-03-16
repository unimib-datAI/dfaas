# SPDX-License-Identifier: AGPL-3.0-or-later

from __future__ import annotations

import json
from pathlib import Path

import pytest

from model import config_constants
from model.model_store import (
    build_manifest,
    build_model_set,
    compute_sha256,
    get_manifest_fingerprint,
    load_manifest,
    validate_checksums,
    validate_manifest,
)
from model.runtime_config import RuntimeConfig


def _write(path: Path, content: bytes) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_bytes(content)


def _make_store(tmp_path: Path) -> Path:
    base_dir = tmp_path / "store"
    for metric in config_constants.METRICS:
        _write(
            base_dir / "models" / metric / "regression" / "model.joblib",
            f"model-{metric}".encode("utf-8"),
        )
        _write(
            base_dir / "scalers" / "scaler_y" / f"{metric}.joblib",
            f"scaler-{metric}".encode("utf-8"),
        )

    _write(
        base_dir / "scalers" / "scaler_x" / "features.joblib",
        b"features-scaler",
    )
    return base_dir


def _write_manifest(base_dir: Path) -> dict[str, str]:
    sha256_map = {}
    for rel_path in [
        "scalers/scaler_x/features.joblib",
        *[
            f"scalers/scaler_y/{metric}.joblib"
            for metric in config_constants.METRICS
        ],
        *[
            f"models/{metric}/regression/model.joblib"
            for metric in config_constants.METRICS
        ],
    ]:
        sha256_map[rel_path] = compute_sha256(base_dir / rel_path)

    manifest = {
        "model_version": "v1",
        "models_type": "regression",
        "created_at": "2026-01-01T00:00:00Z",
        "sha256": sha256_map,
    }
    (base_dir / "manifest.json").write_text(json.dumps(manifest), encoding="utf-8")
    return manifest


def test_validate_manifest_missing_fields():
    errors = validate_manifest({})
    assert "missing 'model_version'" in errors
    assert "missing 'models_type'" in errors
    assert "missing 'created_at'" in errors
    assert "missing 'sha256'" in errors


def test_validate_manifest_invalid_sha256():
    errors = validate_manifest(
        {
            "model_version": "v1",
            "models_type": "regression",
            "created_at": "2026-01-01T00:00:00Z",
            "sha256": [],
        }
    )
    assert "'sha256' must be an object" in errors


def test_validate_manifest_empty_sha256():
    errors = validate_manifest(
        {
            "model_version": "v1",
            "models_type": "regression",
            "created_at": "2026-01-01T00:00:00Z",
            "sha256": {},
        }
    )
    assert "'sha256' must not be empty" in errors


def test_validate_manifest_invalid_sidecar():
    errors = validate_manifest(
        {
            "model_version": "v1",
            "models_type": "regression",
            "created_at": "2026-01-01T00:00:00Z",
            "sha256": {"file": "deadbeef"},
            "sidecar": {"poll_interval_seconds": 0},
        }
    )
    assert "'sidecar.poll_interval_seconds' must be a positive integer" in errors


def test_load_manifest_invalid_json(tmp_path):
    manifest_path = tmp_path / "manifest.json"
    manifest_path.write_text("{invalid}", encoding="utf-8")
    with pytest.raises(json.JSONDecodeError):
        load_manifest(str(manifest_path))


def test_load_manifest_missing(tmp_path):
    with pytest.raises(FileNotFoundError):
        load_manifest(str(tmp_path / "missing.json"))


def test_validate_checksums_missing_and_mismatch(tmp_path):
    base_dir = _make_store(tmp_path)
    manifest = _write_manifest(base_dir)

    missing_path = "models/cpu_usage_node/regression/model.joblib"
    (base_dir / missing_path).unlink()
    manifest["sha256"]["scalers/scaler_x/features.joblib"] = "deadbeef"

    with pytest.raises(ValueError) as exc:
        validate_checksums(str(base_dir), manifest)
    message = str(exc.value)
    assert "missing" in message
    assert "mismatched" in message


def test_manifest_fingerprint_missing(tmp_path):
    assert get_manifest_fingerprint(str(tmp_path / "manifest.json")) is None


def test_build_model_set_success(tmp_path, monkeypatch):
    base_dir = _make_store(tmp_path)
    _write_manifest(base_dir)

    monkeypatch.setattr("model.model.joblib.load", lambda _path: object())

    config = RuntimeConfig(
        models_base_dir=str(base_dir),
        models_type="regression",
        manifest_filename="manifest.json",
        reload_mode="none",
        reload_interval_seconds=30,
        reload_token=None,
    )
    model_set = build_model_set(config)

    assert model_set.model_version == "v1"
    assert model_set.models_type == "regression"
    assert set(model_set.models.keys()) == set(config_constants.METRICS)


def test_build_model_set_model_type_mismatch(tmp_path):
    base_dir = _make_store(tmp_path)
    _write_manifest(base_dir)

    config = RuntimeConfig(
        models_base_dir=str(base_dir),
        models_type="quantile095",
        manifest_filename="manifest.json",
        reload_mode="none",
        reload_interval_seconds=30,
        reload_token=None,
    )

    with pytest.raises(ValueError) as exc:
        build_model_set(config)
    assert "MODELS_TYPE does not match" in str(exc.value)


def test_build_manifest_includes_required_files(tmp_path):
    base_dir = _make_store(tmp_path)
    manifest = build_manifest(
        str(base_dir),
        "regression",
        "v1",
        "2026-01-01T00:00:00Z",
    )

    expected_paths = {
        "scalers/scaler_x/features.joblib",
        *{
            f"scalers/scaler_y/{metric}.joblib"
            for metric in config_constants.METRICS
        },
        *{
            f"models/{metric}/regression/model.joblib"
            for metric in config_constants.METRICS
        },
    }
    assert set(manifest["sha256"].keys()) == expected_paths


def test_build_manifest_with_sidecar(tmp_path):
    base_dir = _make_store(tmp_path)
    manifest = build_manifest(
        str(base_dir),
        "regression",
        "v1",
        "2026-01-01T00:00:00Z",
        sidecar_poll_interval_seconds=25,
    )
    assert manifest["sidecar"]["poll_interval_seconds"] == 25
