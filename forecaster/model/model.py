# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import os
import joblib
import pandas as pd
import numpy as np
from model import config_constants


class Model:
    def __init__(self, metric, model_type, base_dir):
        self.metric = metric
        self.model_type = model_type

        # Load model
        model_path = os.path.join(
            base_dir, "models", metric, model_type, "model.joblib"
        )
        if not os.path.isfile(model_path):
            raise FileNotFoundError(f"Model file not found: {model_path}")
        self.model = joblib.load(model_path)

        # Load features and target scalers
        features_scaler_path = os.path.join(
            base_dir, "scalers", "scaler_x", "features.joblib"
        )
        if not os.path.isfile(features_scaler_path):
            raise FileNotFoundError(
                f"Features scaler not found: {features_scaler_path}"
            )
        self.features_scaler = joblib.load(features_scaler_path)

        target_scaler_path = os.path.join(
            base_dir, "scalers", "scaler_y", metric + ".joblib"
        )
        if not os.path.isfile(target_scaler_path):
            raise FileNotFoundError(f"Target scaler not found: {target_scaler_path}")
        self.target_scaler = joblib.load(target_scaler_path)

    def predict(self, input_data):
        # Scale input dataset
        input_data_scaled = self.features_scaler.transform(input_data)

        # Transform input dataset to dataframe
        input_data_scaled = pd.DataFrame(input_data_scaled,
                                         columns=[*config_constants.GROUPS_COLUMNS_NAMES, "node_type"])
        scaled_predictions = self.model.predict(input_data_scaled)
        original_predictions = self.target_scaler.inverse_transform(scaled_predictions.reshape(-1, 1))
        return np.round(original_predictions, 2)



