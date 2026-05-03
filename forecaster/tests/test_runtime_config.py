# SPDX-License-Identifier: AGPL-3.0-or-later

from pathlib import Path

import pytest

from model.runtime_config import from_config


def test_from_config_loads_file(tmp_path):
    config_path = tmp_path / "runtime.json"
    config_path.write_text(
        """
        {
          "models_base_dir": ".",
          "models_type": "regression",
          "manifest_filename": "manifest.json",
          "reload_mode": "none",
          "reload_interval_seconds": 15,
          "reload_token": null
        }
        """,
        encoding="utf-8",
    )

    config = from_config(config_path, allow_env_overrides=False)
    assert config.models_base_dir == "."
    assert config.models_type == "regression"
    assert config.manifest_filename == "manifest.json"
    assert config.reload_mode == "none"
    assert config.reload_interval_seconds == 15
    assert config.reload_token is None


def test_from_config_missing_file(tmp_path):
    missing_path = tmp_path / "missing.json"
    with pytest.raises(FileNotFoundError):
        from_config(missing_path, allow_env_overrides=False)


def test_from_config_env_override(tmp_path, monkeypatch):
    config_path = tmp_path / "runtime.json"
    config_path.write_text(
        """
        {
          "models_base_dir": ".",
          "models_type": "regression",
          "manifest_filename": "manifest.json",
          "reload_mode": "none",
          "reload_interval_seconds": 15,
          "reload_token": null
        }
        """,
        encoding="utf-8",
    )
    monkeypatch.setenv("RELOAD_MODE", "endpoint")
    config = from_config(config_path, allow_env_overrides=True)
    assert config.reload_mode == "endpoint"
