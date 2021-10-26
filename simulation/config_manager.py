import os

class ConfigManager(object):
    """
        Class used to create/read configuration file.
    """

    _config_manager = None  # Singletoon instance of this class.

    # Constants used in instance generator script
    OUTPUT_INSTANCE_PATH = "instance_gen_output/"
    OUTPUT_INSTANCE_JSON_FILE_PATH = OUTPUT_INSTANCE_PATH + "instance.json" # Path for output instance json file
    OUTPUT_INSTANCE_GRAPH_FILE_PATH = OUTPUT_INSTANCE_PATH + "graph.png"
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
    SIMULATION_COMPLETE_CONFIGURATION_OUTPUT_PATH = "simulation_output/final_config/"
    SIMULATION_TABLES_OUTPUT_PATH = "simulation_output/reports/"
    SIMULATION_AGENT_LOGGING_BASE_PATH = "simulation_output/logs/"

    # Constant used in analyzer
    ANALYSIS_PERCENTILE = 90
    INDEX_TO_COMPARE = [
        "Mean success rate",
        "Success rate variance",
        "Success rate median",
        "Success rate {}% percentile".format(ANALYSIS_PERCENTILE),
        "Mean success rate (stress period)",
        "Success rate variance (stress period)",
        "Success rate median stress period",
        "Success rate {}% percentile stress period".format(ANALYSIS_PERCENTILE),
        "Tot. rejected requests",
        "Reject num variance",
        "Reject num median",
        "Reject num {}% percentile".format(ANALYSIS_PERCENTILE),
    ]
    ANALYZER_OUTPUT_PATH = "analyzer_output/"
    INDEX_COMPARISON_FILE = ANALYZER_OUTPUT_PATH + "index_comparison.csv"

    # Constant used in simulation controller
    NUMBER_OF_SIMULATION_EXECUTION = 5
    SIMULATION_CONTROLLER_OUTPUT_PATH = "simulation_controller_output/"
    SIMULATION_CONTROLLER_OUTPUT_FILE = SIMULATION_CONTROLLER_OUTPUT_PATH + \
                                        "experiment_results.csv"

    # Constant used for store resume of simulation controller
    SIMULATION_CONTROLLER_ARCHIVE_PATH = "archive/"
    SIMULATION_CONTROLLER_ARCHIVE_COMPARISON_FILE_NAME = "final_comparison.txt"

    def __new__(self):
        # if is not define create new instance otherwise return only instance of thi class.
        if not isinstance(self._config_manager, self):
            self._config_manager = object.__new__(self)
        return self._config_manager