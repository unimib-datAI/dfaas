# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import os
import json
from pathlib import Path


class ConfigManager(object):
    """
        Class used to create/read configuration file.
    """

    _config_manager = None  # Singletoon instance of this class.

    root_path = Path("../").resolve()
    simulation_dir = root_path.joinpath("simulation")
    output_dir = simulation_dir.joinpath("outputs")

    # Constants used in instance generator script
    OUTPUT_INSTANCE_PATH = output_dir.joinpath("instance_gen_output")
    OUTPUT_INSTANCE_JSON_FILE_PATH = OUTPUT_INSTANCE_PATH.joinpath("instance.json") # Path for output instance json file
    OUTPUT_INSTANCE_GRAPH_FILE_PATH = OUTPUT_INSTANCE_PATH.joinpath("graph.png")
    DATA_DIR = simulation_dir.joinpath("data")  # Directory that contains experiment files

    NODES_TYPES = ["HEAVY", "MID", "LIGHT"] # Nodes types used in experiments
    NODES_TYPE_PROBABILITY_DISTRIBUTION = [0.2, 0.3, 0.5]
    NODES_TYPES_IN_MODELS = {"HEAVY": 0, "MID": 1, "LIGHT": 2} # Association of the nodes types with the values used in the models
    PREDICTED_METRICS = ["cpu_usage_node", "overloaded_node", "power_usage_node", "ram_usage_node"] # Metrics predicted by the forecaster
    MODEL_TYPES = ["quantile005", "quantile095", "regression"]
    NODE_KEY_PREFIX = "node_"

    FUNCTION_GROUPS = 4 # Number of groups of functions
    GROUP_KEY_PREFIX = "group_"
    GROUPS_MAX_RATES = [70, 200, 70, 50] # Max rates for each group of function
    INITIAL_LOAD_STEPS = 2 # Number of instances for increasing load
    MODEL_BASE_PATH = '../metrics_predictions/system-forecaster-models/groups/'
    SCALER_BASE_PATH = '../metrics_predictions/scalers/groups/'
    GROUP_FILE_PATH = '../metrics_predictions/group_list.json' # group_list path
    
    # Read group_list file
    with open(GROUP_FILE_PATH, 'r') as json_file:
        groups_number = json.load(json_file)

    # Map groups number with the correspondent name
    GROUPS = {}
    for key, value in groups_number.items():
        if "figlet" in value:
            GROUPS["LOW_USAGE"] = value
        elif "nmap" in value:
            GROUPS["HIGH_USAGE"] = value
        else:
            GROUPS["MEDIUM_USAGE"] = value

    # All the functions present in the config file 
    FUNCTION_NAMES = []

    # Name of rates columns
    GROUPS_COLUMNS_NAMES = []
    for group, functions in GROUPS.items():
        GROUPS_COLUMNS_NAMES.append('rate_group_' + group)
        for function in functions:
            FUNCTION_NAMES.append(function)
    
    # Parameters for Node Margin Strategy
    MAX_RESOURCES_USAGE = {"cpu_usage_node": {"HEAVY": 460, "MID": 290, "LIGHT": 150}, 
                            "ram_usage_node": {"HEAVY": 6000000000, "MID": 5500000000, "LIGHT": 4100000000}, 
                            "power_usage_node": {"HEAVY": 3500000, "MID": 2200000, "LIGHT": 700000}}
    
    # Parameters for Power Saving Strategy
    MAX_POWER_USAGE = {node_type: value * 0.70 for node_type, value in MAX_RESOURCES_USAGE["power_usage_node"].items()}

    # Constants used in simulation script
    SIMULATION_MINUTES = 7

    STRATEGIES = ["base_strategy", "equal_strategy", "node_margin_strategy", "power_saving_strategy"]
    BASE_STRATEGY = STRATEGIES[0]
    EQUAL_STRATEGY = STRATEGIES[1]
    NODE_MARGIN_STRATEGY = STRATEGIES[2]
    POWER_SAVING_STRATEGY = STRATEGIES[3]

    SIMULATION_OUTPUT_DIR = output_dir.joinpath("simulation_output")
    SIMULATION_COMPLETE_CONFIGURATION_OUTPUT_PATH = SIMULATION_OUTPUT_DIR.joinpath("minute_config")
    SIMULATION_TABLES_OUTPUT_PATH = SIMULATION_OUTPUT_DIR.joinpath("reports")
    SIMULATION_AGENT_LOGGING_BASE_PATH = SIMULATION_OUTPUT_DIR.joinpath("logs")

    # Constant used in analyzer
    ANALYSIS_PERCENTILE = 90
    INDEX_TO_COMPARE = [
        "Mean success rate",
        "Success rate median",
        "Mean success rate (stress period)",
        "Success rate median (stress period)",
        "Mean node power consumption",
        "Max node power consumption",
        "Tot. rejected requests"
    ]

    ANALYZER_OUTPUT_PATH = output_dir.joinpath("analyzer_output")
    INDEX_COMPARISON_FILE = ANALYZER_OUTPUT_PATH.joinpath("index_comparison.csv")

    # Constant used in simulation controller
    NUMBER_OF_SIMULATION_EXECUTION = 5
    SIMULATION_CONTROLLER_OUTPUT_PATH = output_dir.joinpath("simulation_controller_output")
    SIMULATION_CONTROLLER_OUTPUT_FILE = SIMULATION_CONTROLLER_OUTPUT_PATH.joinpath("experiment_results.csv")
    SIMULATION_CONTROLLER_OUTPUT_MEAN_FILE = SIMULATION_CONTROLLER_OUTPUT_PATH.joinpath("experiment_mean_results.csv")

    # Constant used for store resume of simulation controller
    SIMULATION_CONTROLLER_ARCHIVE_PATH = output_dir.joinpath("archive")
    SIMULATION_CONTROLLER_ARCHIVE_COMPARISON_FILE_NAME = "final_comparison.txt"

    def __new__(self):
        # if is not define create new instance otherwise return only instance of thi class.
        if not isinstance(self._config_manager, self):
            self._config_manager = object.__new__(self)
        return self._config_manager
