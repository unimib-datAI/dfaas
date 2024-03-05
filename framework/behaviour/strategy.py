from __future__ import annotations
from abc import ABC, abstractmethod
from configuration.config_manager import ConfigManager
from model.model_proxy import ModelProxy

class Strategy(ABC):
    def __init__(self, config_file):
        self._config_json = config_file
        self._config_manager = ConfigManager()
        
        # Model manager is passed from simulation context
        self._model_proxy = None

    @abstractmethod
    def run(self) -> dict:
        pass

    def set_model_proxy(self, model_manager: ModelProxy):
        self._model_proxy = model_manager
    
    # Get load for each group and function in the node reading the config file
    def _get_load_for_groups_and_functions(self, config_json):
        groups_requests = {}
        functions_requests = {}
        for k, value in config_json["load"].items():
            if "USAGE" in k:
                groups_requests[k] = value["total_rate"]
                for i in range(0, len(value["functions"])):
                    functions_requests[value["functions"][i]["function_name"]] = value["functions"][i]["function_rate"]
        return groups_requests, functions_requests
    
    # Get the name of the group of the function passed
    def _get_group_of_function(self, function_name):
        for group, functions in self._config_manager.GROUPS.items():
            if function_name in functions:
                return group
