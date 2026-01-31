# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

from dataclasses import replace
import threading

from model import config_constants
from model.model_store import build_model_set, get_manifest_fingerprint, ModelSet
from model.runtime_config import RuntimeConfig
import pandas as pd


class ModelProxy:
    def __init__(self, config: RuntimeConfig):
        self._config = config
        self._model_type = config.models_type
        self._lock = threading.Lock()
        self._model_set: ModelSet | None = None
        self._last_error: str | None = None
        self._last_manifest_fingerprint: str | None = None

    def set_model_type(self, model_type):
        self._model_type = model_type

    def get_model_type(self):
        return self._model_type

    def load_models(self, strict: bool = True) -> ModelSet | None:
        if not self._model_type:
            self._last_error = "MODELS_TYPE is not set. Aborting model load."
            if strict:
                raise RuntimeError(self._last_error)
            return None
        config = replace(self._config, models_type=self._model_type)
        try:
            model_set = build_model_set(config)
            fingerprint = get_manifest_fingerprint(config.manifest_path)
            with self._lock:
                self._model_set = model_set
                self._last_error = None
                self._last_manifest_fingerprint = fingerprint
            return model_set
        except Exception as exc:  # noqa: BLE001
            self._last_error = str(exc)
            if strict:
                raise
            return None

    def reload_if_manifest_changed(self) -> bool:
        config = replace(self._config, models_type=self._model_type)
        new_fingerprint = get_manifest_fingerprint(config.manifest_path)
        if new_fingerprint is None:
            self._last_error = f"Manifest file not found: {config.manifest_path}"
            return False
        if new_fingerprint == self._last_manifest_fingerprint:
            return False
        return self.load_models(strict=False) is not None

    def get_predictions(self, input_data, metric, to_json=False):
        input_data_df = self._json_to_df_input_data(input_data)
        model = self._get_model(metric)
        predictions = pd.DataFrame(index=range(0, 1))
        if to_json is True:
            predictions[metric] = model.predict(input_data_df)
            predictions_json = self._df_to_json_predictions(predictions)
            return predictions_json
        else:
            predictions = model.predict(input_data_df)
            return predictions

    def get_node_predictions(self, input_data):
        predictions = pd.DataFrame(index=range(0, 1))
        for metric in config_constants.METRICS:
            predictions[metric] = self.get_predictions(input_data, metric)
        predictions_json = self._df_to_json_predictions(predictions)
        return predictions_json

    def _get_model(self, metric):
        model_set = self._model_set
        if not model_set:
            raise RuntimeError("Models are not loaded.")
        try:
            return model_set.get(metric)
        except KeyError as exc:
            raise RuntimeError("Requested model not found.") from exc

    def _json_to_df_input_data(self, input_data):
        input_data_df = pd.json_normalize(input_data)
        return input_data_df

    def _df_to_json_predictions(self, predictions):
        predictions_json = predictions.to_dict(orient="records")
        return predictions_json

    def models_loaded_count(self):
        model_set = self._model_set
        return len(model_set.models) if model_set else 0

    def model_version(self) -> str | None:
        model_set = self._model_set
        return model_set.model_version if model_set else None

    def manifest_ok(self) -> bool:
        return self._model_set is not None

    def last_error(self) -> str | None:
        return self._last_error
