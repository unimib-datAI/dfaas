# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

from model.model import Model
from model import config_constants
import pandas as pd


class ModelProxy:
    _models = []

    def __init__(self, model_type):
        self._model_type = model_type

    def set_model_type(self, model_type):
        self._model_type = model_type

    def create_model(self, metric):
        model = Model(metric, self._model_type)
        self._models.append(model)

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
        for model in self._models:
            if model.metric == metric and model.model_type == self._model_type:
                return model
        raise Exception("Requested model not found.")

    def _json_to_df_input_data(self, input_data):
        input_data_df = pd.json_normalize(input_data)
        return input_data_df

    def _df_to_json_predictions(self, predictions):
        predictions_json = predictions.to_dict(orient="records")
        return predictions_json
