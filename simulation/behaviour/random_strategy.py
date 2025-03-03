# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import json
import numpy as np
from .strategy import Strategy
from configuration.config_manager import ConfigManager

class RandomStrategy(Strategy):
    def __init__(self, config_json):
        self._config_json = config_json
        self._config_manager = ConfigManager()

    def run(self) -> dict:
        return self.loop()

    def loop(self) -> dict:
        self._data = self._config_json
        
        weights = {}
        for func in self._data[self._id]["functions"]:
            if func["name"] in self._config_manager.FUNCTION_NAMES and func["state"] == "Overload":
                self._logger.debug("FUNC: " + func["name"] + " is OVERLOADED")
                
                weights[func["name"]] = {}
                for node, val in self._data.items():
                    if node != self._id:
                        weights[func["name"]][node] = np.random.randint(1, 101)
                
                #self._logger.debug("Weights not normalized for func {}".format(func["name"]))
                #self._logger.debug(weights[func["name"]])
                
                weights[func["name"]] = self.recalc_distribution(weights[func["name"]])

                self._logger.debug("Weights normalized for func {}".format(func["name"]))
                self._logger.debug(weights[func["name"]])
                
                #self._logger.info("Weights normalized for func {}".format(func["name"]))
                #self._logger.info(weights[func["name"]])
                self._logger.info("Weights normalized for func {}: {}".format(
                    func["name"], weights[func["name"]]))

        return weights

    def set_id(self, id):
        self._id = self._config_manager.NODE_KEY_PREFIX + str(id)
        self._prefix = "THREAD: " + self._id

    def set_logger(self, logger):
        self._logger = logger

    def recalc_distribution(self, w):
        return {k: (v / sum(w.values()))*100 for k, v in w.items()}
        
