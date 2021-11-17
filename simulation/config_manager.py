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
    DATA_DIR = "../data/"  # Directory that contains experiment files
    NODES_TYPE = ["node_1", "node_2", "node_3"] # Nodes type used in experiments
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
    INDEX_TO_MEAN_FOR_COMPARISON_FOR_TXT_FILE = [
        "Mean success rate",
        "Success rate variance",
        "Mean success rate (stress period)",
        "Success rate variance (stress period)",
        "Tot. rejected requests",
        "Reject num variance",
    ]

    # Constant used for store resume of simulation controller
    SIMULATION_CONTROLLER_ARCHIVE_PATH = "archive/"
    SIMULATION_CONTROLLER_ARCHIVE_COMPARISON_FILE_NAME = "final_comparison.txt"

    # DB path
    EXPERIMENT_DB_PATH = "database_design/experiment_db"

    # Mapping info
    NODE_CONFIGURATIONS = {
        "node_1": {
            "ram": 2,
            "cpu": 1,
        },
        "node_2": {
            "ram": 4,
            "cpu": 2,
        },
        "node_3": {
            "ram": 6,
            "cpu": 4,
        },
    }

    FUNCTIONS_DESCRPTION = {
        "funca": "Dummy function that returns an 'Hello World' string \
                    containing the name of the node that had processed the request.",
        "qrcode": "Function which uses the skip2/go-qrcode Go library to generate a QR Code for a string. \
                    See: https://github.com/faas-and-furious/qrcode",
        "ocr": "Optical character recognition function. See: https://github.com/viveksyngh/openfaas-ocr"
    }

    NODES_METRICS = {
        "ram_usage": "RAM usage of the node in a specific moment of the experiment.",
        "cpu_usage": "CPU usage of the node in a specific moment of the experiment."
    }

    NODES_METRICS_UNIT = {
        "ram_usage": "%",
        "cpu_usage": "%"
    }

    FUNCTION_METRICS = {
        "service_count": "Number of replicas.",
        "margin": "Number of requests (in a second) that a function can still serve",
        "invoc_rate": "Number of requests (in a second) received by a functions",
        "afet": "Average function execution time (in seconds).",
        "ram_xfunc": "Ram usage of a function.",
        "cpu_xfunc": "CPU usage of a function.",
        "max_rate": "Maximum number of requests that a function can handle.",
        "state": "State of a function.",
        "prom_invoc_rate": "Invocation rate gathered by prometheus (number of requests actually served)."
    }

    FUNCTION_METRICS_UNIT = {
        "service_count": "",
        "state": "",
        "margin": "req/sec",
        "invoc_rate": "req/sec",
        "max_rate": "req/sec",
        "prom_invoc_rate": "req/sec",
        "afet": "",
        "ram_xfunc": "%",
        "cpu_xfunc": "%"
    }

    DEPLOY_DATAS = ["service_count", "invoc_rate", "margin", "state"]

    def __new__(self):
        # if is not define create new instance otherwise return only instance of thi class.
        if not isinstance(self._config_manager, self):
            self._config_manager = object.__new__(self)
        return self._config_manager
