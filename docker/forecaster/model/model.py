import os
import joblib
import pandas as pd
import numpy as np
from model import config_constants


class Model:
    def __init__(self, metric, model_type):
        self.metric = metric
        self.model_type = model_type

        # Load model
        self.model = joblib.load(os.path.join(config_constants.MODELS_BASE_PATH,
                                              metric, model_type, "model.joblib"))

        # Load features and target scalers
        self.features_scaler = joblib.load(os.path.join(config_constants.SCALERS_BASE_PATH,
                                                        "scaler_x", "features.joblib"))
        self.target_scaler = joblib.load(os.path.join(config_constants.SCALERS_BASE_PATH,
                                                      "scaler_y", metric + ".joblib"))

    def predict(self, input_data):
        # Scale input dataset
        input_data_scaled = self.features_scaler.transform(input_data)

        # Transform input dataset to dataframe
        input_data_scaled = pd.DataFrame(input_data_scaled,
                                         columns=[*config_constants.GROUPS_COLUMNS_NAMES, "node_type"])
        scaled_predictions = self.model.predict(input_data_scaled)
        original_predictions = self.target_scaler.inverse_transform(scaled_predictions.reshape(-1, 1))
        return np.round(original_predictions, 2)





