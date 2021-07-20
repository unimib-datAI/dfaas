import os
import json
import numpy as np
from enum import Enum
from threading import Thread

class MetricType(Enum):
    DIRECT = 1
    INVERT = 2

class Agent(): # Inherit by Thread in () bratches
    
    ANALYTICS = {
        "service_count" : MetricType.DIRECT,
        "invoc_rate"    : MetricType.INVERT,
        "afet"          : MetricType.INVERT,
        "margin"        : MetricType.DIRECT,
        "ram_usage"     : MetricType.INVERT,
        "cpu_usage"     : MetricType.INVERT,
        "ram_xfunc"     : MetricType.INVERT,
        "cpu_xfunc"     : MetricType.INVERT,
    }

    # Default values are useful only when there is a 0 value in an INVERT analytic
    ANALYTICS_DEFAULT_VALUES = {
        "service_count" : 0, # DIRECT -- no risk of divison by zero
        "invoc_rate"    : 1, # INVERTED
        "afet"          : 0.001, # INVERTED
        "margin"        : "max-rate - invoc_rate", # DIRECT -- no risk of division by zero
        "ram_usage"     : 0.01,  # 1% -- INVERTED
        "cpu_usage"     : 0.01,  # 1% -- INVERTED
        "ram_xfunc"     : 0.01,  # 1% -- INVERTED
        "cpu_xfunc"     : 0.01,  # 1% -- INVERTED
    }

    _json_path = ""
    
    def __init__(self, id, file, logger):
        super().__init__()
        self._id = "node_" + str(id)
        self._json_path = self._json_path + file
        self._prefix = "THREAD: " + self._id
        self._logger = logger

    def run(self):
        self.loop()

    def loop(self):
        self.monitor()
        self.exchange()
        w = self.analyze()
        w = self.plan(w)
        self.execute(w)

    '''
        Mocked: instead of reading informations directly 
        from the cluster read info from a json file with the 
        complete configuration.
    '''    
    def monitor(self):
        self._logger.info("=======================")
        self._logger.info("1. MONITOR")
        self._logger.info("=======================")
        f = open(self._json_path)
        self._data = json.load(f) # Return json file as a dictionary

        self._logger.info("======== Read data ========")
        self._logger.info(self._data)
        self._logger.info("=======================")
        # self._logger.info(self._data["node_" + str(self._id)])
        # self._logger.info("=======================")
        # self._logger.info(self._data["node_" + str(self._id)]["functions"][0])
        # self._logger.info("=======================")
        # self._logger.info(self._data["node_" + str(self._id)]["functions"][0]["afet"])

    '''
        Mocked: communication in this simulation is not a key point.
    '''
    def exchange(self):
        self._logger.info("=======================")
        self._logger.info("2. EXCHANGE (pass)")
        self._logger.info("=======================")
        pass

    '''
        Not mocked: but it is one of the most tricky part of the algorithm.
    '''
    def analyze(self):
        self._logger.info("=======================")
        self._logger.info("3. ANALYZE")
        self._logger.info("=======================")
        ws = self.analytics_weights()
        self._logger.info("======== Metric weights ========")
        self._logger.info(ws)
        w = self.weights_aggregation(ws)
        self._logger.info("======== Aggregated weights ========")
        self._logger.info(w)
        return w

    '''
        Not mocked: applies probabilistic strategy and add rumor to weights.
    '''
    def plan(self, w):
        self._logger.info("=======================")
        self._logger.info("4. PLAN")
        self._logger.info("=======================")
        for func, node_weights in w.items():
            self._logger.info("Weights for func " + func)
            for node, wi in node_weights.items():
                self._logger.info("Node " + node + " -- w = " + str(wi))
                
                rnd = np.random.uniform(0, 1)
        
                # Probability of 60% to add rumor
                if rnd > 0.6:
                    continue

                # Rumor between 0% and 40% wrt weight value
                rumor = np.random.uniform(0, 0.4) * wi
                rnd = np.random.uniform(0, 1)
                
                # Probability of 50% to add rumor
                # and 50% to subtract rumor
                if rnd >= 0.5:
                    w[func][node] = wi + rumor
                else:
                    w[func][node] = wi - rumor
            
            self._logger.info("-----")
            
            # Recalc distribution of weight for func[x] towards all other nodes
            w[func] = self.recalc_distribution(w[func])
        
        self._logger.info("======== Final Weights ========")
        self._logger.info(w)
        
        self._logger.info("======== Final Weights Sum ========")
        for func, val in w.items():
            self._logger.info("Sum for func " + func + " = " + str(sum(val.values())))

        return w

    '''
        Not mocked: write weights on configuration file.
    '''
    def execute(self, w):
        self._logger.info("=======================")
        self._logger.info("5. EXECUTE")
        self._logger.info("=======================")
        
        # This is loop used for add weights for all nodes that not has
        # an "underloaded" function (need to have 0% of weight)
        for node in self._data:
            if node == self._id:
                continue

            for func, val in w.items():
                w_nodes = list(val.keys())
                if not node in w_nodes:
                    w[func][node] = 0
        
        # Remember to return weigths that sum to 100 (not 1)
        self._logger.info("======== Before moltiplication ========")
        self._logger.info(w)                

        # Transform weight to obtain 100 as sum
        for func in w:
            w[func] = {k: v * 100 for k, v in w[func].items()}

        self._logger.info("======== After moltiplication ========")
        self._logger.info(w)
        
        self._logger.info("======== Final Weights Sum ========")
        for func, val in w.items():
            self._logger.info("Sum for func " + func + " = " + str(sum(val.values())))

    """
        Compute weight for single analytics.
    """
    def analytics_weights(self):
        ws = {} # Map with key "overloaded" functions name and as value a nested map
                # The nested map has as key node_id and as value helpers functions map
                # Three nested map
        
        # TODO: Try to optimize saving "overloaded" functions in advance and then use the list of them
        # The complexity seems to be the same
        # If "overloaded" functions are found in advance another iteration on them is required

        # For each "overloaded" func on this node
        for func in self._data[self._id]["functions"]:
            if func["state"] == "overloaded":
                self._logger.info("FUNC: " + func["name"] + " is OVERLOADED")
                helpers = {}
                
                # Iterate over json dictionary to found node that can help overloaded functions
                for node, val in self._data.items():
                    self._logger.info("Check NODE: " + node)
                    # This is the same node that search for help --> skip
                    if node == self._id:
                        self._logger.info(" > Skip -- same node")
                        continue
                    
                    # Check "underload" same functions on other nodes
                    for f in val["functions"]:
                        if f["name"] == func["name"] and f["state"] == "underloaded":
                            # RAM and CPU usage are added to node informations
                            f["ram_usage"] = val["ram_usage"]
                            f["cpu_usage"] = val["cpu_usage"]
                            # This node can help the overloaded function
                            helpers[node] = f # Save it on helper node

                            # After that the helper function has been found in helper node, 
                            # it is impossible that there will be another function with the same name
                            break

                # Call a method that truly calculate analytics weight based on helpers node
                # For each function call weight calciulation
                ws[func["name"]] = self.compute_weight(helpers)
            else:
                self._logger.info("FUNC: " + func["name"] + " is UNDERLOADED")

        #self._logger.info("======== WS MAP ========")
        #self._logger.info(ws)  # Three nested map
        return ws

    """
        Method that calculate weight for a specific "overloaded" function to
        all other nodes and for each analytics.
            - "functions" param represent the map containing info of helpers functions.
    """
    def compute_weight(self, functions):
        self._logger.info("======== Helpers functions on other nodes: ========")
        self._logger.info(functions)

        ws = {} # Map that has as key node_id and as values a nested map
                # The nested map has as key the metric name and as value the analytic's weight

        for node in functions:
            ws[node] = {}

        # Iterate over all analytics
        for metric, rel in self.ANALYTICS.items():
            den = 0
            w_metric = {}

            # For each tuple (node, function) caculate sum of anlytics on each node
            # Depending on the metric/analytic type use DIRECT or INVERTED formula
            # for analytics weight
            for node, values in functions.items():
                den += values[metric]
                if metric == "invoc_rate":
                    if values[metric] != 0:
                        # Express the utilization rate
                        w_metric[node] = values[metric] / values["max_rate"]
                    else:
                        # Note: if invocation rate is 0 the next division is not correct
                        # because 1 as default value is used. Instead need to be used
                        # 1 (default) / max_rate. This is only in case of "invoc_rate" metric 
                        w_metric[node] = self.ANALYTICS_DEFAULT_VALUES[metric] / \
                            values["max_rate"]
                        
                        # Print useful for debug
                        # if self._id == "node_0":
                        #     print(node)
                        #     print(w_metric[node])
                        #     print(self.ANALYTICS_DEFAULT_VALUES[metric])
                        #     print(values["max_rate"])
                else:
                    w_metric[node] = values[metric]

            if rel == MetricType.DIRECT:
                w_metric = {k: v / den for k, v in w_metric.items()}
            else:
                # Note: there will be an error for metrics that has value = 0
                w_metric = {k: 1 / v if v != 0 else 1 /
                            self.ANALYTICS_DEFAULT_VALUES[metric] for k, v in w_metric.items()}
                den = sum(w_metric.values())
                w_metric = {k: v / den for k, v in w_metric.items()}

            for node, weight in w_metric.items():
                ws[node][metric] = weight

        return ws

    """
        Aggregate weights for single metrics in a unique weight for each function and for each node
    """
    def weights_aggregation(self, ws):
        weights = {}

        for func, values in ws.items():
            weights[func] = {} # Initialize a map for each func-node tuple

            for node in values:
                h = np.ones((1, len(values[node].values())))        # h could be used to different weigthing of features
                weights[func][node] = sum([i*j for i, j in zip(values[node].values(), h[0])])
                
        # Probability distribution
        for func in weights:
            weights[func] = {k: v / sum(weights[func].values()) for k, v in weights[func].items()}

        self._logger.info(weights)
        for func, val in weights.items():
            self._logger.info("Sum for function " + func + " = " + str(sum(val.values())))
        return weights
        
    def recalc_distribution(self, w):
        return {k: v / sum(w.values()) for k, v in w.items()}
