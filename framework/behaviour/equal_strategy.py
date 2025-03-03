# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

from .strategy import Strategy
import copy
import math

class EqualStrategy(Strategy):

    def __init__(self, config_json):
        super().__init__(config_json)

    def run(self) -> dict:
        return self.loop()


    def loop(self) -> dict:
        weights = {}

        # Extract loads data of the node from the config file
        original_requests_groups, original_requests_functions = self._get_load_for_groups_and_functions(self._config_json)

        # Add the information about the type of the node
        original_requests_groups["node_type"] = self._config_manager.NODES_TYPES_IN_MODELS[self._config_json["node_type"]]

        # Get a copy for each original dictionary
        remained_requests_groups = copy.deepcopy(original_requests_groups)          
        remained_requests_functions = copy.deepcopy(original_requests_functions)

        # List of functions deployed on the node
        node_functions = list(original_requests_functions.keys())

        # Iterator of the functions in the node
        i = 0

        # Iterate removing load until the node is underload
        overload = self._config_json["node_metrics"]["overloaded_node"]

        load_discard_percentage = 0.01
        while overload == 1:

            # Remove a percentage of the requests for each function > 0 
            for i in range(0, len(node_functions)):
                if remained_requests_functions[node_functions[i]] > 0:
                    # Calculate the rate to remove for the selected function
                    load_to_discard = math.ceil(load_discard_percentage * remained_requests_functions[node_functions[i]])
                    
                    # Remove the rate of the function from the node
                    remained_requests_functions[node_functions[i]] -= load_to_discard

                    # Remove the rate group of the selected function
                    group_to_discard = self._get_group_of_function(node_functions[i])
                    remained_requests_groups[group_to_discard] -= load_to_discard

            #Get prediction using the new features
            node_predictions = self._model_proxy.get_node_predictions(remained_requests_groups)

            # Check if the node is still in overload
            overload = node_predictions["overloaded_node"].iloc[0]

            # Reset the index of the functions
            i += 1
            if i == len(node_functions):
                i = 0

        # It contains, for each functions, the removed and the mantained requests
        rem_mant_request_function = {}

        for func in node_functions:
            rem_mant_request_function[func] = {}
            rem_mant_request_function[func]["removed"] = original_requests_functions[func] - remained_requests_functions[func]
            rem_mant_request_function[func]["mantained"] = remained_requests_functions[func]
                

        n_of_neigh = len(self._config_json["neighbours"])
        
        for fun in rem_mant_request_function.keys():
            weights[fun] = {}
            # Obtain total number of requests of the selected function
            tot_n_of_req = rem_mant_request_function[fun]["removed"] + rem_mant_request_function[fun]["mantained"]
            # Use divmod to get the quotient and remainder
            quotient, remainder = divmod(rem_mant_request_function[fun]["removed"], n_of_neigh)
            for node in self._config_json["neighbours"] + [self._id]:
                if tot_n_of_req == 0:
                    weights[fun][node] = 0.0
                else:
                    # If node is a neighbour
                    if node != self._id:
                        weights[fun][node] = round((quotient + remainder) / tot_n_of_req, 4)
                        remainder = 0
                    else:
                        weights[fun][node] = round(rem_mant_request_function[fun]["mantained"] / tot_n_of_req, 4)
        
        for func, val in weights.items():
            self._logger.info("Weights for func {}: {}".format(func, val))

        return weights

    def set_id(self, id):
        self._id = self._config_manager.NODE_KEY_PREFIX + str(id)
        self._prefix = "THREAD: " + self._id

    def set_logger(self, logger):
        self._logger = logger
