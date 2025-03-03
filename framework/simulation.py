# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import logging
import time
import json
import pandas as pd
import numpy as np
from agent.agent import Agent
from configuration.config_manager import ConfigManager
from factory.strategy_factory import StrategyFactory
from model.model_proxy import ModelProxy
from cli.cli import get_analyzer_and_simulator_args
import os

config_manager = ConfigManager()

# Create the specific path if not present in the machine    
def create_path_if_not_exists(path):
    if not os.path.exists(path):
        os.makedirs(path)
    return path

# Get a specific logger with passed configurations
def get_logger(name, log_file, level=logging.DEBUG):
    """
    Get logger for agent logging
    """
    handler = logging.FileHandler(log_file)
    logger = logging.getLogger(name)
    logger.setLevel(level)
    logger.addHandler(handler)

    return logger

def complete_fwd_table(weights, invoc_rate_table):

    fwd_requests = {}

    # Transform the weights in requests by applying weights * invoc_rate_table
    for node_from, weights_x_func in weights.items():
        fwd_requests[node_from] = {}
        for func, weights_x_node in weights_x_func.items():
            fwd_requests[node_from][func] = {}
            for node_to, weight in weights_x_node.items():
                fwd_requests[node_from][func][node_to] = int(weight * invoc_rate_table[node_from][func])
        

    # Fill forwarding table with missing functions
    for node_from, weights_x_func in fwd_requests.items():
        for f in config_manager.FUNCTION_NAMES:
            if f not in list(weights_x_func.keys()):
                fwd_requests[node_from][f] = {}

    # Set with all the nodes of the simulation
    nodes_set = set(fwd_requests.keys())  

    # Complete the forwarding table with all the missing nodes to forward requests
    for node_from, weights_x_func in fwd_requests.items():
        for func, weights_x_node in weights_x_func.items():
            if func in config_manager.FUNCTION_NAMES:
                for node in nodes_set:
                    if node not in list(weights_x_node.keys()):
                        fwd_requests[node_from][func][node] = 0
    return fwd_requests

def create_tables(fwd_requests, minute, strategy_type):
    """
    Starting by forwarding requests create a table and export it in a CSV file
    Also invocation rate and max rate table are create and exported in the same 
    format
    """
    path = config_manager.SIMULATION_TABLES_OUTPUT_PATH.joinpath(strategy_type, "minute_" + str(minute))
    create_path_if_not_exists(path)
    nodes_set = sorted(set(fwd_requests.keys()))

    # Foreach function and for each node create a dataframe with forwarded requests
    for func in config_manager.FUNCTION_NAMES:
        df_x_func = pd.DataFrame([], index=nodes_set)
        for node_from in fwd_requests:
            df_x_func[node_from] = [fwd_requests[node_from][func][k]
                                    for k in sorted(fwd_requests[node_from][func].keys())]
        # Invert rows and columns
        df_x_func = df_x_func.T
        df_x_func.to_csv(path.joinpath(func + ".csv"), sep='\t', encoding='utf-8')

        print("     > FWD_TABLE FOR FUNC {}".format(func))
        print(df_x_func)
        

def run_agent(agent):
    """
    Run agent loop, calculate execution time and return it along with weights
    """
    # time.perf_counter() returns elapsed time in seconds
    # It is the best way to measure performance
    #
    # See: https://www.geeksforgeeks.org/time-perf_counter-function-in-python/
    start = time.perf_counter()
    weights = agent.run()
    end = time.perf_counter()
    execution = end - start

    return weights, execution


def simulation(nodes_number, config_file, model_type):
    """
    This function allow to simulate various strategies for workload distribution
    and use weights to distribuite the load across neighbours
    """
    model_proxy = ModelProxy()
    model_proxy.set_model_type(model_type)

    # Execution time dictionary
    execution_times = {}

    # Initialize execution time map for each strategy
    for s in config_manager.STRATEGIES:
        execution_times[s] = []

    for minute in range(0, config_manager.SIMULATION_MINUTES):  # 6 minutes
        # Dictionary that contains final json configuration of the current minute
        minute_config = {}

        # Dictionaries used for export
        simulation_weights_table = {}
        simulation_invoc_rate_table = {}        

        # Forwarding requests dictionary
        fwd_requests = {}

        # Initialize maps for each strategy
        for s in config_manager.STRATEGIES:
            simulation_weights_table[s] = {}
            fwd_requests[s] = {}

        # Create global configuration file with info of all nodes
        for i in range(0, nodes_number):
            key = config_manager.NODE_KEY_PREFIX + str(i)
            minute_config[key] = {}

            # Add the information about the node type and the neighbours of the current node
            minute_config[key]["node_type"] = config_file[key]["node_type"]
            minute_config[key]["neighbours"] =  config_file[key]["neighbours"]

            # Add the information about the load of the current minute
            minute_config[key]["load"] = config_file[key]["load"][minute]

            simulation_invoc_rate_table[key] = {}

            features_data = {}
            for k, value in minute_config[key]["load"].items():
                # Extracts the load of each group
                features_data[k] = value["total_rate"]

                # Iterate over the functions of the group
                for j in range(0, len(minute_config[key]["load"][k]["functions"])):
                    # Populate the dicitonary which contains the load of each function deployed on the node
                    simulation_invoc_rate_table[key][value["functions"][j]["function_name"]] = value["functions"][j]["function_rate"]

            # Insert the information about the node type in the features dict
            features_data["node_type"] = config_manager.NODES_TYPES_IN_MODELS[minute_config[key]["node_type"]]

            # Get node predictions for the selected features
            node_predictions = model_proxy.get_node_predictions(features_data)

            # Add node metrics to minute_config
            minute_config[key]["node_metrics"] = node_predictions.to_dict(orient='records')[0]



        print("----------------------------------------------------------")
        

        with open(create_path_if_not_exists(config_manager.SIMULATION_COMPLETE_CONFIGURATION_OUTPUT_PATH).joinpath(
                  'config{}.json'.format(minute)), 'w', encoding='utf-8') as f:
            json.dump(minute_config, f, ensure_ascii=False, indent=4)


        # Call agent loop for each config that has been previously built
        #
        # With last update this code is executed for each type of behaviour
        # (base, random and empirical) and for each agent in the network
        for id in range(0, nodes_number):
            key = config_manager.NODE_KEY_PREFIX + str(id)
            # config_with_neigh = {}
            # config_with_neigh[key] = minute_config[key] # Add this node
            # neighbours = config_file[key]["neighbours"]

            # # Create configuration file with only neighbours
            # for neighbour in neighbours:
            #     config_with_neigh[neighbour] = minute_config[neighbour]

            logger = get_logger(
                "agent" + str(id) + "_minute_" + str(minute),
                create_path_if_not_exists(config_manager.SIMULATION_AGENT_LOGGING_BASE_PATH).joinpath("agent_" +
                str(id) + ".log"),
                logging.INFO
            )

            logger.info("\n")
            logger.info("-------- MINUTE {} --------".format(minute))

            # Execute agent loop for each strategy
            for s in config_manager.STRATEGIES:
                # Build correct strategy
                strategy = StrategyFactory.create_strategy(s, key, minute_config)
                logger.info("   > STRATEGY: {} <".format(s))
                agent = Agent(
                    id,
                    logger,
                    strategy,
                    model_proxy
                )
                #agent.disable_logging() # Disable logging for speed
                weights, execution_time = run_agent(agent)
                execution_times[s].append(execution_time)
                simulation_weights_table[s][key] = weights
        for s in config_manager.STRATEGIES:
            fwd_requests[s] = complete_fwd_table(simulation_weights_table[s], simulation_invoc_rate_table)

        print("> START MINUTE {}".format(minute))

        for s in config_manager.STRATEGIES:
            # Create and export tables for three algorithms
            print(" > {}".format(s))
            create_tables(fwd_requests[s], minute, s)
            print("------------------------------------------------")

        print("> END MINUTE {}".format(minute))

    return {k: np.mean(times_for_algo) for k, times_for_algo in execution_times.items()}


def main(instance_file=""):
    kargs = get_analyzer_and_simulator_args()
    model_type = kargs["modeltype"]

    # Instance file can come from simulation_controller
    if instance_file == "":
        instance_file = config_manager.OUTPUT_INSTANCE_JSON_FILE_PATH

    # Read the instance json produced by the instance generator
    f = open(instance_file)
    config_file = json.load(f)
    simulation(config_file["nodes_number"], config_file, model_type)



# Call main program.
if __name__ == "__main__":
    main()
