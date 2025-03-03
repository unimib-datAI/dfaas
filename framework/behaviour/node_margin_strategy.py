# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

from .strategy import Strategy
import copy
import math


class NodeMarginStrategy(Strategy):
    def __init__(self, config_json):
        super().__init__(config_json)

        # For each node represents the percentage of usage that the neighbours can use
        self._margin_percentage_in = {}

        # Represents the percentage of usage that is possible forward to each neighbour
        self._margin_percentage_out = {}

    def run(self) -> dict:
        return self.__loop()

    def set_id(self, id):
        self._id = self._config_manager.NODE_KEY_PREFIX + str(id)
        self._prefix = "THREAD: " + self._id

    def set_logger(self, logger):
        self._logger = logger

    def __loop(self) -> dict:
        self._calculate_margin()
        self._exchange()
        return self._calculate_weights()
    
    # Get all the functions deployed in the selected node
    def __get_deployed_functions(self, actual_node):
        functions_deployed = []
        for k, value in self._config_json[actual_node]["load"].items():
            if "USAGE" in k:
                for i in range(0, len(value["functions"])):
                    functions_deployed.append(value["functions"][i]["function_name"])
        return functions_deployed
    
    # Get the number of neigh with at least one function in common with the actual node
    def __neigh_with_at_least_one_fun_in_common(self, actual_node):
        functions_deployed = self.__get_deployed_functions(actual_node)

        num = 0
        for neigh in self._config_json[actual_node]["neighbours"]:
            found = False
            for k, value in self._config_json[neigh]["load"].items():
                if "USAGE" in k:
                    for i in range(0, len(value["functions"])):
                        if value["functions"][i]["function_name"] in functions_deployed:
                            num += 1
                            found = True
                            break
                if found == True:
                    break
        return num
                        
    # Calculates the margin for each node in the net
    def _calculate_margin(self):
        for node, node_infos in self._config_json.items():
            self._margin_percentage_in[node] = {}
            self._margin_percentage_in[node]["node_type"] = node_infos["node_type"]
            # In case the node is in overload, the only information that produces is margin = 0
            # avoiding to send information about the load 
            if node_infos["node_metrics"]["overloaded_node"] == 0:
                # Get the load of each group in the node
                self._margin_percentage_in[node]["load"] = {}
                for group, values in node_infos["load"].items():
                    self._margin_percentage_in[node]["load"][group] = values["total_rate"]
                neigh_num = self.__neigh_with_at_least_one_fun_in_common(node)
                ##print(neigh_num)
                if neigh_num == 0:
                    self._margin_percentage_in[node]["margin"] = 0
                else:
                    node_type = node_infos["node_type"]
                    # Calculate the percentage usage for each node metric
                    usage_percentage = {}
                    skip_calculations = False
                    for metric, values in self._config_manager.MAX_RESOURCES_USAGE.items():
                        # If at least one metric is higher than the considered max value, then the margin is 0
                        if node_infos["node_metrics"][metric] > values[node_type]:
                            self._margin_percentage_in[node]["margin"] = 0
                            skip_calculations = True
                            break
                        else:
                            usage_percentage[metric] = (node_infos["node_metrics"][metric] * 100) / values[node_type]
                    ##print(usage_percentage)
                    # Skip the next calculations if at least one metric was higher than the considered max value
                    if skip_calculations:
                        continue

                    # Obtain a single value of the node usage mediating over all the node metrics
                    total_usage_percentage = 0
                    for metric in usage_percentage:
                        total_usage_percentage += usage_percentage[metric]
                    total_usage_percentage /= len(usage_percentage.keys())
                    ##print(total_usage_percentage)
                    
                    # Obtain the margin of the node
                    self._margin_percentage_in[node]["margin"] = round((100 - total_usage_percentage) / neigh_num, 4)
            else:
                self._margin_percentage_in[node]["margin"] = 0
            
    def _exchange(self):
        """
        Mocked phase of algo. All agents already has all information to execute weights calculation
        During this phase "margin_percentage_in" are exchanged from each node to other neighbours
        Note: during previous phase "margin_percentage_in" are calculated for each node so the exchange phase only transform margin_percentage_in in margin_percentage_out
        """

        for node, limits in self._margin_percentage_in.items():
            if self._id == node or not node in self._config_json[self._id]["neighbours"]:
                continue
            self._margin_percentage_out[node] = {}
            for k, v in limits.items():
                self._margin_percentage_out[node][k] = v

        logger_dict = {}
        for node, values in self._margin_percentage_out.items():
            logger_dict[node] = values["margin"]
        self._logger.info("Margin percentage out: {}".format(logger_dict))

    def _calculate_weights(self):
        w = {}

        # Extract loads data of the node from the config file
        original_requests_groups, original_requests_functions = self._get_load_for_groups_and_functions(self._config_json[self._id])

        # Add the information about the type of the node
        original_requests_groups["node_type"] = self._config_manager.NODES_TYPES_IN_MODELS[self._config_json[self._id]["node_type"]]

        # Get a copy for each original dictionary
        remained_requests_groups = copy.deepcopy(original_requests_groups)          
        remained_requests_functions = copy.deepcopy(original_requests_functions)

        # List of functions deployed on the node
        node_functions = list(original_requests_functions.keys())

        # For each neighbour with margin > 0, list of the functions deployed in common with the current node
        # If a neighbour does not have any function in common, it is not considered
        functions_in_common = self.__get_common_functions(node_functions)
        #print("Functions in common:", functions_in_common)

        # List of the neighbours available to receive load
        neighbours = list(functions_in_common.keys())

        # For each neighbour and for each function in common, contains the load forwarded by the current node
        fwd_to_neigh = {}
        for neigh, list_of_fun in functions_in_common.items():
            fwd_to_neigh[neigh] = {}
            for i in range(0, len(list_of_fun)):
                fwd_to_neigh[neigh][list_of_fun[i]] = 0

        #print("Fwd to neigh original", fwd_to_neigh)

        # Iterator for:
        # - Selected neighbour
        # - Selected function for each neighbour
        iterator = {}
        iterator["neigh"] = 0
        for i in range (0, len(neighbours)):
            iterator[neighbours[i]] = 0
        # Iterate removing load until the node is underload
        overload = self._config_json[self._id]["node_metrics"]["overloaded_node"]
        while overload == 1:
            # Check if there's at least one neigh who can still receive load for some function
            zero_neigh_available = True
            for node_to in functions_in_common:
                if len(functions_in_common[node_to]) != 0:
                    zero_neigh_available = False
                    break
            
            # If there're not neigh who can receive load, even if the node is in overload, it cannot transfer any request
            if not zero_neigh_available:
                # Get the name of the function and the node selected by the iterator
                node_to = list(functions_in_common.keys())[iterator["neigh"]]
                func_to = functions_in_common[node_to][iterator[node_to]]

                # Check if there're still requests to remove from the selected function
                if remained_requests_functions[func_to] > 0:
                    # Extracts the load of the selected neighbour (this information has been forwarded toghether with the information about the margin) 
                    original_node_to_features = copy.deepcopy(self._margin_percentage_out[node_to]["load"])
                    # Add node type 
                    original_node_to_features["node_type"] = self._config_manager.NODES_TYPES_IN_MODELS[self._margin_percentage_out[node_to]["node_type"]]

                    original_node_to_predictions = self._model_proxy.get_node_predictions(original_node_to_features)

                    # Calculate the percentage usage for each node metric
                    original_node_to_percentage = self.__calculate_usage_node(original_node_to_predictions, self._margin_percentage_out[node_to]["node_type"])
                    #print("Original node" + node_to + " percentage: " + str(original_node_to_percentage))
                    
                    # Calculate load to transfer
                    load_to_transfer = math.ceil(remained_requests_functions[func_to] * 0.01)
                    # Check if the selected "node_to" can receive the load
                    fwd_to_neigh[node_to][func_to] += load_to_transfer
                    #print("Fwd to neigh after trying to send a new request", fwd_to_neigh)

                    # Get the correspondent group data of the current load chosen to be forwarded at "node_to"
                    group_data = self._model_proxy.transform_functions_in_groups(fwd_to_neigh[node_to])
                    # Add the original load present in "node_to"
                    for group in self._margin_percentage_out[node_to]["load"]:
                        group_data["rate_group_" + group].iloc[0] += self._margin_percentage_out[node_to]["load"][group]

                    # Add the information about the node type (it is forwarded togheter with the margin percentage)
                    group_data["node_type"] = self._config_manager.NODES_TYPES_IN_MODELS[self._margin_percentage_out[node_to]["node_type"]]

                    #Get prediction using the new features
                    node_to_predictions = self._model_proxy.get_node_predictions(group_data)

                    # Calculate the percentage usage for each node metric
                    node_to_percentage = self.__calculate_usage_node(node_to_predictions, self._margin_percentage_out[node_to]["node_type"])
                    #print("New percentage after adding load:" + str(node_to_percentage))
                    # Check if "node_to" can receive the request
                    if self._margin_percentage_out[node_to]["margin"] >= node_to_percentage - original_node_to_percentage:
                        remained_requests_functions[func_to] -= load_to_transfer
                        #print("New load after removing one request:", remained_requests_functions)
                        group_to_discard = self._get_group_of_function(func_to)
                        remained_requests_groups[group_to_discard] -= load_to_transfer
                        #Get prediction using the new features
                        node_predictions = self._model_proxy.get_node_predictions(remained_requests_groups)

                        # Check if the current node is still in overload
                        overload = node_predictions["overloaded_node"].iloc[0]
                        #print("Is the node still in overload?:" + str(overload))
                    else:
                        # Cancel the transfer of the request
                        fwd_to_neigh[node_to][func_to] -= 1

                        # Node_to cannot accept requests from func_to so remove the function from the possibilities
                        functions_in_common[node_to].remove(func_to)
                        # Reset the iterator of the functions of "Node_to"
                        iterator[node_to] = -1
                        # If node_to has no more functions available, delete node_to from the possibilities
                        if len(functions_in_common[node_to]) == 0:
                            del functions_in_common[node_to]
                            iterator["neigh"] = -1

                # Since the node has no more requests of the selected function to forward
                # remove it from the possibilities of each neigh
                else:
                    remained_nodes = list(functions_in_common.keys())
                    for node in remained_nodes:
                        if func_to in functions_in_common[node]:
                            functions_in_common[node].remove(func_to)
                            iterator[node] = -1

                            if len(functions_in_common[node]) == 0:
                                del functions_in_common[node]
                                iterator["neigh"] = -1
                
                # Reset iterator
                iterator["neigh"] += 1
                if iterator["neigh"] >= len(list(functions_in_common.keys())):
                    iterator["neigh"] = 0
                if iterator[node_to] != -1:
                    iterator[node_to] += 1
                    if iterator[node_to] == len(functions_in_common[node_to]):
                        iterator[node_to] = 0
            else:
                break
        
        for fun in node_functions:
            w[fun] = {}
            # Obtain total number of requests of the selected function
            tot_n_of_req = original_requests_functions[fun]

            for node in self._config_json[self._id]["neighbours"] + [self._id]:
                if tot_n_of_req == 0:
                    w[fun][node] = 0.0
                else:
                    if node != self._id:
                        if node in list(fwd_to_neigh.keys()) and fun in list(fwd_to_neigh[node].keys()):
                            w[fun][node] = round(fwd_to_neigh[node][fun] / tot_n_of_req, 4)
                        else:
                            w[fun][node] = 0.0
                    else:
                        w[fun][node] = round(remained_requests_functions[fun] / tot_n_of_req, 4)
            
        self._logger.info(w)
        for func, val in w.items():
            self._logger.info("Weights for func {}: {}".format(func, val))
        return w
    
    # Calculate functions in common with each node in the net
    def __get_common_functions(self, current_node_functions):
        common_functions = {}
        for node in self._config_json:
            if node == self._id or not node in self._config_json[self._id]["neighbours"] or self._margin_percentage_out[node]["margin"] == 0:
                continue
            node_functions = self.__get_deployed_functions(node)
            temp_common_functions = list(set(node_functions).intersection(current_node_functions))
            if len(temp_common_functions) > 0:
                common_functions[node] = temp_common_functions
        return common_functions
    
    # Calculate the percentage usage for each node metric
    def __calculate_usage_node(self, predictions, node_type):
        usage_percentage = {}
        fwd_req = True
        for metric, values in self._config_manager.MAX_RESOURCES_USAGE.items():
            # If at least one metric is higher than the considered max value, then the request cannot be forwarded
            if predictions[metric].iloc[0] > values[node_type]:
                fwd_req = False
                break
            else:
                usage_percentage[metric] = (predictions[metric].iloc[0] * 100) / values[node_type]
        if fwd_req:
            # Obtain a single value of the node usage mediating over all the node metrics
            total_usage_percentage = 0
            for metric in usage_percentage:
                total_usage_percentage += usage_percentage[metric]
            total_usage_percentage /= len(usage_percentage.keys())
            return total_usage_percentage
        else:
            return 100


