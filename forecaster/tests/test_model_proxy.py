# SPDX-License-Identifier: AGPL-3.0-or-later

import pandas as pd

from model.model_proxy import ModelProxy
from model import config_constants


class _FakeModel:
    def __init__(self, metric, model_type, value):
        self.metric = metric
        self.model_type = model_type
        self._value = value

    def predict(self, _input):
        return self._value


def test_get_predictions_json(monkeypatch):
    proxy = ModelProxy("regression")
    fake = _FakeModel(config_constants.CPU_USAGE_METRIC, "regression", [1.23])
    monkeypatch.setattr(ModelProxy, "_models", [fake])

    result = proxy.get_predictions({"a": 1}, config_constants.CPU_USAGE_METRIC, True)
    assert isinstance(result, list)
    assert result == [{config_constants.CPU_USAGE_METRIC: 1.23}]


def test_get_node_predictions_aggregates(monkeypatch):
    proxy = ModelProxy("regression")
    models = [
        _FakeModel(config_constants.CPU_USAGE_METRIC, "regression", [1.11]),
        _FakeModel(config_constants.RAM_USAGE_METRIC, "regression", [2.22]),
        _FakeModel(config_constants.POWER_USAGE_METRIC, "regression", [3.33]),
    ]
    monkeypatch.setattr(ModelProxy, "_models", models)

    result = proxy.get_node_predictions({"a": 1})
    assert result == [
        {
            config_constants.CPU_USAGE_METRIC: 1.11,
            config_constants.RAM_USAGE_METRIC: 2.22,
            config_constants.POWER_USAGE_METRIC: 3.33,
        }
    ]


def test_get_model_missing_raises(monkeypatch):
    proxy = ModelProxy("regression")
    monkeypatch.setattr(ModelProxy, "_models", [])

    try:
        proxy.get_predictions({"a": 1}, config_constants.CPU_USAGE_METRIC)
        assert False, "Expected exception for missing model."
    except Exception as exc:  # noqa: BLE001
        assert "Requested model not found" in str(exc)


def test_json_to_df_input_data():
    proxy = ModelProxy("regression")
    df = proxy._json_to_df_input_data({"a": 1, "b": 2})
    assert isinstance(df, pd.DataFrame)
    assert df.to_dict(orient="records") == [{"a": 1, "b": 2}]


def test_models_loaded_count(monkeypatch):
    proxy = ModelProxy("regression")
    monkeypatch.setattr(ModelProxy, "_models", [object(), object(), object()])
    assert proxy.models_loaded_count() == 3
