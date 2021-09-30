import os

class ConfigManager(object):
    """
        Class used to create/read configuration file.
    """

    _config_manager = None  # Singletoon instance of this class.
    
    OUTPUT_INSTANCE_JSON_FILE_PATH = "instance/instance.json" # Path for output instance json file
    OUTPUT_INSTANCE_GRAPH_FILE_PATH = "instance/graph.png"
    DATA_DIR = "../data/" # Directory that contains experiment files
    NODES_TYPE = ["node1", "node2", "node3"] # Nodes type used in experiments
    NODES_TYPE_PROBABILITY_DISTRIBUTION = [0.5, 0.3, 0.2]

    def __new__(self):
        # if is not define create new instance otherwise return only instance of thi class.
        if not isinstance(self._config_manager, self):
            self._config_manager = object.__new__(self)
        return self._config_manager
