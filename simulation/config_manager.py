import os

class ConfigManager(object):
    """
        Class used to create/read configuration file.
    """

    _config_manager = None  # Singletoon instance of this class.

    # Constants used in instance generator script
    OUTPUT_INSTANCE_JSON_FILE_PATH = "instance/instance.json" # Path for output instance json file
    OUTPUT_INSTANCE_GRAPH_FILE_PATH = "instance/graph.png"
    DATA_DIR = "../data/" # Directory that contains experiment files
    NODES_TYPE = ["node1", "node2", "node3"] # Nodes type used in experiments
    NODES_TYPE_PROBABILITY_DISTRIBUTION = [0.5, 0.3, 0.2]
    NODE_KEY_PREFIX = "node_"

    # Constants used in simulation script
    SIMULATION_MINUTES = 7
    FUNCTION_NAMES = ["funca", "qrcode", "ocr"]
    STRATEGIES = ["base_strategy", "random_strategy", "empirical_strategy"]
    BASE_STRATEGY = STRATEGIES[0]
    RANDOM_STRATEGY = STRATEGIES[1]
    EMPIRICAL_STRATEGY = STRATEGIES[2]
    SIMULATION_COMPLETE_CONFIGURATION_OUTPUT_PATH = "test/final_config/"
    SIMULATION_TABLES_OUTPUT_PATH = "test/reports/"
    SIMULATION_AGENT_LOGGING_BASE_PATH = "test/logs/"

    # Constant used in analyzer
    INDEX_TO_COMPARE = [
        "Mean success rate",
        "Mean success rate (stress period)",
        "Tot. rejected requests"
    ]
    ANALYZER_OUTPUT_PATH = "analyzer_output/"

    def __new__(self):
        # if is not define create new instance otherwise return only instance of thi class.
        if not isinstance(self._config_manager, self):
            self._config_manager = object.__new__(self)
        return self._config_manager
