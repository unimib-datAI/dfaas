# SPDX-License-Identifier: AGPL-3.0-or-later

import numpy as np
import pandas as pd

from model.model import Model
from model import config_constants


class _FakeFeatureScaler:
    def transform(self, input_data):
        assert isinstance(input_data, pd.DataFrame)
        return input_data.to_numpy()


class _FakeTargetScaler:
    def inverse_transform(self, data):
        return data + 10


class _FakeModel:
    def __init__(self):
        self.seen_columns = None

    def predict(self, data):
        self.seen_columns = list(data.columns)
        return np.array([1.2345])


def test_model_predict_rounding_and_columns(monkeypatch):
    fake_model = _FakeModel()
    load_calls = []

    def _fake_joblib_load(path):
        load_calls.append(path)
        if len(load_calls) == 1:
            return fake_model
        if len(load_calls) == 2:
            return _FakeFeatureScaler()
        return _FakeTargetScaler()

    monkeypatch.setattr("model.model.joblib.load", _fake_joblib_load)

    model = Model(config_constants.CPU_USAGE_METRIC, "regression")
    input_data = pd.DataFrame(
        [[1, 2, 3, "edge"]],
        columns=[*config_constants.GROUPS_COLUMNS_NAMES, "node_type"],
    )
    predictions = model.predict(input_data)

    expected_columns = [*config_constants.GROUPS_COLUMNS_NAMES, "node_type"]
    assert fake_model.seen_columns == expected_columns
    assert predictions.shape == (1, 1)
    assert predictions[0, 0] == 11.23


def test_model_missing_files_raise(monkeypatch):
    def _fake_isfile(_path):
        return False

    monkeypatch.setattr("model.model.os.path.isfile", _fake_isfile)

    try:
        Model(config_constants.CPU_USAGE_METRIC, "regression")
        assert False, "Expected FileNotFoundError for missing model files."
    except FileNotFoundError as exc:
        assert "not found" in str(exc)
