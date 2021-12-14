import numpy as np
from .strategy import Strategy
from configuration.config_manager import ConfigManager


class DFaasStaticStrategy(Strategy):

    def __init__(self, config_json):
        self._config_json = config_json
        self._config_manager = ConfigManager()

        # For each node represents number of requests that can be received from others node (for each function)
        self._limit_in = {}
        # Represents number of request that is possible forward to other nodes, for each function
        self._limit_out = {}

    def run(self) -> dict:
        return self.__loop()

    def set_id(self, id):
        self._id = self._config_manager.NODE_KEY_PREFIX + str(id)
        self._prefix = "THREAD: " + self._id

    def set_logger(self, logger):
        self._logger = logger

    def __loop(self) -> dict:
        """
        Static algorithm implemented as first strategy in DFaaS
        Thi algo does not consider exporters data
        """
        self.__phase1()
        self.__exchange()
        return self.__phase2()

    def __neighbours_with_this_func(self, actual_node, func_name):
        """
        Calculates nodes that has this functions that can send request to actual_node
        :actual_node: name of considered node
        :func_name: name of considered function
        :return: number of nodes that has func_name
        """
        num = 0
        for node, metrics in self._config_json.items():
            if node == actual_node:
                continue
            for func in metrics["functions"]:
                # Note: does not check if in other nodes function is in "Underload" state
                # because are number of neigh from which I can receive requests in next step
                if func["name"] == func_name:
                    num += 1
                    break  # Max 1 function with the same name on the same node
        return num

    def __phase1(self):
        """
        Phase 1 steps:
            1) Gather information from proxy and FaaS cluster
            2) Get max_rates for of cluster's functions
            3) Define, for each function, its state ("Overload" or "Normal"/"Underload")
            4) For each function in "Normal" state, calculates "limit_in"
        Note: data already has function state, so step 1 and 2 can be skipped
        Limit_in calculation is done for each node instead of communication phase
        """
        # print(self._config_json)

        # A dictionary containing limit in for each node, for each function is created
        # ex:
        #       "node_0": {
        #           "funca" : 3,
        #           "qrcode": 4,
        #       }, "node_1" : {}
        for node, metrics in self._config_json.items():
            self._limit_in[node] = {}
            for func in metrics["functions"]:
                if func["name"] in self._config_manager.FUNCTION_NAMES:
                    neigh_num = self.__get_neighbours_with_this_func(node, func["name"])
                    margin = func["max_rate"] - func["invoc_rate"] if func["state"] == "Underload" else 0
                    # Calculate how much request can be served for neighbours
                    self._limit_in[node][func["name"]] = margin / neigh_num if neigh_num > 0 else 0

            self._logger.debug("Limits_in: {}".format(self._limit_in))

    def __exchange(self):
        """
        Mocked phase of algo. All agents already has all information to execute weights calculation
        During this phase "limit_in" are exchanged from each node to other neighbours
        Note: during previous phase "limit_in" are calculated for each node so the exchange phase only transform limit_in
        in limit_out
        """
        # limit_in:
        #       "node_0": {
        #           "funca" : 3,
        #           "qrcode": 4,
        #       },
        #       "node_1" : {
        #           "funca" : 5,
        #           "qrcode": 6,
        #       }
        #
        # limit_out:
        #       "funca": {
        #           "node_0" : 3,
        #           "node_1" : 5,
        #       },
        #       "qrcode" : {
        #           "node_0" : 4,
        #           "node_1" : 6,
        #       }
        #
        for func in self._config_manager.FUNCTION_NAMES:
            self._limit_out[func] = {}

        for node, limits in self._limit_in.items():
            if self._id == node:
                continue
            for func, limit in limits.items():
                self._limit_out[func][node] = limit

        self._logger.debug("Limits_out: {}".format(self._limit_out))

    def __phase2(self):
        """
        Phase 2 steps:
            1) For each "overloaded" calculate weights for forwarding requests, using "limit_in" previously received
            2) Write weights on HAproxy config file
        """
        w = {}

        #for func in self._config_manager.FUNCTION_NAMES:
        #    w[func] = {}

        # For the specified node calculate weights
        for func in self._config_json[self._id]["functions"]:
            if func["name"] in self._config_manager.FUNCTION_NAMES and func["state"] == "Overload":
                w[func["name"]] = {}
                for node, limit_out in self._limit_out[func["name"]].items():
                    if sum(self._limit_out[func["name"]].values()) > 0:
                        # Calculate weights for HA-proxy, based on limit_out, number
                        # of requests that can be forwarded to other nodes
                        # limits_out correspond to limit_in received after phase1() (calculated in exchange)
                        w[func["name"]][node] = limit_out / sum(self._limit_out[func["name"]].values()) * 100
                    else:
                        w[func["name"]][node] = 0.0

        for func, val in w.items():
            self._logger.info("Weights normalized for func {}: {}".format(func, val))

        return w
