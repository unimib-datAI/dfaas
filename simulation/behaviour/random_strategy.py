import json
import numpy as np
from .strategy import Strategy

class RandomStrategy(Strategy):
    _json_path = ""

    def __init__(self, file, to_be_loaded=True, config_json=None):
        #self._id = "node_" + str(id)
        self._json_path = self._json_path + file
        #self._prefix = "THREAD: " + self._id
        #self._logger = logger
        self._to_be_loaded = to_be_loaded
        self._config_json = config_json

    def run(self) -> dict:
        return self.loop()

    '''
        M(E)APE control loop
    '''

    def loop(self) -> dict:
        if self._to_be_loaded:
            f = open(self._json_path)
            self._data = json.load(f)  # Return json file as a dictionary
        else:
            self._data = self._config_json
        
        weights = {}
        for func in self._data[self._id]["functions"]:
            if func["state"] == "Overload":
                self._logger.info("FUNC: " + func["name"] + " is OVERLOADED")
                
                weights[func["name"]] = {}
                for node, val in self._data.items():
                    if node != self._id:
                        weights[func["name"]][node] = np.random.randint(0, 101)
                
                self._logger.info("Weights not normalized for func {}".format(func["name"]))
                self._logger.info(weights[func["name"]])
                
                weights[func["name"]] = self.recalc_distribution(weights[func["name"]])

                self._logger.info("Weights normalized for func {}".format(func["name"]))
                self._logger.info(weights[func["name"]])

        return weights

    def set_id(self, id):
        self._id = "node_" + str(id)
        self._prefix = "THREAD: " + self._id

    def set_logger(self, logger):
        self._logger = logger

    def recalc_distribution(self, w):
        return {k: (v / sum(w.values()))*100 for k, v in w.items()}
        
