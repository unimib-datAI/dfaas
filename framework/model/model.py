# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import joblib
import pandas as pd
import numpy as np

from configuration.config_manager import ConfigManager

class Model:

    __config_manager = ConfigManager()

    def __init__(self, metric, model_type):
        self.metric = metric
        self.model_type = model_type

        # Load the model by the joblib file produced by the forecaster
        self.model = joblib.load(self.__config_manager.MODEL_BASE_PATH + metric + "/" + model_type + "/model.joblib")

        # For the overloaded metric, load only the scaler of the features
        if "overloaded" in metric:
            self.features_scaler = joblib.load(self.__config_manager.SCALER_BASE_PATH + "scaler_x/" + metric + ".joblib")
            self.target_scaler = None
        else:
            self.features_scaler = joblib.load(self.__config_manager.SCALER_BASE_PATH + "scaler_x/features.joblib")
            self.target_scaler = joblib.load(self.__config_manager.SCALER_BASE_PATH + "scaler_y/" + metric + ".joblib")

    def predict(self, input_data):
        # Scale input dataset
        input_data_scaled = self.features_scaler.transform(input_data)

        # Transform input dataset to dataframe
        input_data_scaled = pd.DataFrame(input_data_scaled, columns=[*self.__config_manager.GROUPS_COLUMNS_NAMES, "node_type"])
        scaled_predictions = self.model.predict(input_data_scaled)
        if "overloaded" in self.metric:
            return scaled_predictions
        else:
            original_predictions = self.target_scaler.inverse_transform(scaled_predictions.reshape(-1,1))
            return np.round(original_predictions,2)
        


        

    
