# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

from configuration.config_manager import ConfigManager
from model.model import Model
import pandas as pd

class ModelProxy:
    """
    Class used as handler of all the models created by the forecaster
    """
    _config_manager = ConfigManager()
    _models = []

    def __init__(self):
        self._model_type = None

    def _create_model(self, metric):
        if "overloaded" not in metric:
            return Model(metric, self._model_type)
        else:
            return Model(metric, "")
        
    def _get_model(self, metric):
        """
        Method used to find a particular model in the list
        :metric: the metric predicted by the model
        """
        for model in self._models:
            if model.metric == metric and ("overloaded" in model.metric or model.model_type == self._model_type): 
                return model
        model = self._create_model(metric)
        self._models.append(model)
        return model
        #raise Exception("It has not been possible to find the requested model") 
        
    
    def _process_input(self, input_data):
        """
        Method used to transform input_data in the correct data structure
        :input_data: data to transform
        """
        input_data_df = pd.DataFrame(index=range(0,1))

        # Populate the df with the load of each group
        for group in self._config_manager.GROUPS:
            if group in input_data:
                input_data_df['rate_group_' + group] = input_data[group]
            else:
                input_data_df['rate_group_' + group] = 0
        
        # Insert the information about the node type in the df
        input_data_df["node_type"] = input_data["node_type"]
        return input_data_df

    def get_predictions(self, input_data, metric):
        """
        Method used get predictions from a particular model
        :input_data: features values
        :metric: the metric predicted by the model
        """
        # Check if the features are already in the correct structure
        if isinstance(input_data, dict):
            input_data_df = self._process_input(input_data)
        else:
            input_data_df = input_data
        return self._get_model(metric).predict(input_data_df)

    def get_node_predictions(self, input_data):
        """
        Method used get predictions of the all node metrics
        :input_data: features values
        """
        predictions = pd.DataFrame(index=range(0,1))
        
        # Iterate over all the metric of the node
        for metric in self._config_manager.PREDICTED_METRICS:
            predictions[metric] = self.get_predictions(input_data, metric)
        return predictions
    

    def transform_functions_in_groups(self, functions_data):
        """
        Method used to transform data from function to group form
        :input_data: data in function form
        """
        groups_data = pd.DataFrame(index=range(0,1))
        for group, functions in self._config_manager.GROUPS.items():
            temp_counter = 0
            for function in functions:
                # This branch of the if is used in the stategies, and it could not contain the selected function
                if isinstance(functions_data, dict):
                    if function in list(functions_data.keys()):
                        temp_counter += functions_data[function]
                # This branch is used in the analyzer, and it contains already all the functions
                else:
                    temp_counter += functions_data.get(function)
            groups_data["rate_group_" + group] = temp_counter
        return groups_data
    
    def set_model_type(self, model_type):
        self._model_type = model_type

