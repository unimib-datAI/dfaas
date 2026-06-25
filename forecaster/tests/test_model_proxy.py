# SPDX-License-Identifier: AGPL-3.0-or-later

import pandas as pd

from model.model_proxy import ModelProxy
from model.model_store import ModelSet
from model.runtime_config import RuntimeConfig
from model import config_constants


class _FakeModel:
    def __init__(self, metric, model_type, value):
        self.metric = metric
        self.model_type = model_type
        self._value = value

    def predict(self, _input):
        return self._value


def _config():
    return RuntimeConfig(
        models_base_dir="/tmp/models",
        models_type="regression",
        manifest_filename="manifest.json",
        reload_mode="none",
        reload_interval_seconds=30,
        reload_token=None,
    )


def test_get_predictions_json(monkeypatch):
    proxy = ModelProxy(_config())
    fake = _FakeModel(config_constants.CPU_USAGE_METRIC, "regression", [1.23])
    proxy._model_set = ModelSet(
        models={config_constants.CPU_USAGE_METRIC: fake},
        model_version="v1",
        models_type="regression",
        manifest={},
    )

    result = proxy.get_predictions({"a": 1}, config_constants.CPU_USAGE_METRIC, True)
    assert isinstance(result, list)
    assert result == [{config_constants.CPU_USAGE_METRIC: 1.23}]


def test_get_node_predictions_aggregates(monkeypatch):
    proxy = ModelProxy(_config())
    models = [
        _FakeModel(config_constants.CPU_USAGE_METRIC, "regression", [1.11]),
        _FakeModel(config_constants.RAM_USAGE_METRIC, "regression", [2.22]),
        _FakeModel(config_constants.POWER_USAGE_METRIC, "regression", [3.33]),
    ]
    proxy._model_set = ModelSet(
        models={
            config_constants.CPU_USAGE_METRIC: models[0],
            config_constants.RAM_USAGE_METRIC: models[1],
            config_constants.POWER_USAGE_METRIC: models[2],
        },
        model_version="v1",
        models_type="regression",
        manifest={},
    )

    result = proxy.get_node_predictions({"a": 1})
    assert result == [
        {
            config_constants.CPU_USAGE_METRIC: 1.11,
            config_constants.RAM_USAGE_METRIC: 2.22,
            config_constants.POWER_USAGE_METRIC: 3.33,
        }
    ]


def test_get_model_missing_raises(monkeypatch):
    proxy = ModelProxy(_config())

    try:
        proxy.get_predictions({"a": 1}, config_constants.CPU_USAGE_METRIC)
        assert False, "Expected exception for missing model."
    except Exception as exc:  # noqa: BLE001
        assert "Models are not loaded" in str(exc)


def test_json_to_df_input_data():
    proxy = ModelProxy(_config())
    df = proxy._json_to_df_input_data({"a": 1, "b": 2})
    assert isinstance(df, pd.DataFrame)
    assert df.to_dict(orient="records") == [{"a": 1, "b": 2}]


def test_models_loaded_count(monkeypatch):
    proxy = ModelProxy(_config())
    proxy._model_set = ModelSet(
        models={"a": object(), "b": object(), "c": object()},
        model_version="v1",
        models_type="regression",
        manifest={},
    )
    assert proxy.models_loaded_count() == 3


def test_load_models_requires_model_type(monkeypatch):
    config = RuntimeConfig(
        models_base_dir="/tmp/models",
        models_type=None,
        manifest_filename="manifest.json",
        reload_mode="none",
        reload_interval_seconds=30,
        reload_token=None,
    )
    proxy = ModelProxy(config)
    result = proxy.load_models(strict=False)
    assert result is None
    assert "MODELS_TYPE" in (proxy.last_error() or "")


def test_reload_if_manifest_changed(monkeypatch):
    proxy = ModelProxy(_config())

    fingerprints = iter(["a", "b"])
    monkeypatch.setattr(
        "model.model_proxy.get_manifest_fingerprint",
        lambda _path: next(fingerprints),
    )

    def _load_models(strict=False):
        proxy._model_set = ModelSet(
            models={},
            model_version="v1",
            models_type="regression",
            manifest={},
        )
        return proxy._model_set

    monkeypatch.setattr(proxy, "load_models", _load_models)

    assert proxy.reload_if_manifest_changed() is True
    assert proxy.reload_if_manifest_changed() is True


def test_reload_if_manifest_unchanged(monkeypatch):
    proxy = ModelProxy(_config())
    proxy._last_manifest_fingerprint = "same"
    monkeypatch.setattr(
        "model.model_proxy.get_manifest_fingerprint",
        lambda _path: "same",
    )
    assert proxy.reload_if_manifest_changed() is False


def test_reload_if_manifest_missing(monkeypatch):
    proxy = ModelProxy(_config())
    monkeypatch.setattr(
        "model.model_proxy.get_manifest_fingerprint",
        lambda _path: None,
    )
    assert proxy.reload_if_manifest_changed() is False
    assert "Manifest file not found" in (proxy.last_error() or "")
