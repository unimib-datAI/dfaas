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
from data_loader.data_loader import DataLoader
from data_loader.request.function_request import FunctionRequest
from data_loader.request.config_request import ConfigRequest

config_manager = ConfigManager()


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


def xfunc_request_table(max_rate_table, invoc_rate_table, weights_table):
    """
    Functions that calculate forwarding requests for each function starting by
    weights dictionary passed as param
    """
    # Utility prints
    # print("============= MAX RATE TABLE =============")
    # print(max_rate_table)
    # print("==========================================")
    # print("============= INVOC RATE TABLE ===========")
    # print(invoc_rate_table)
    # print("==========================================")
    # print("============= WEIGHTS TABLE ==============")
    # print(weights_table)
    # print("==========================================")
    
    fwd_requests = {}
    for node_from, weights_x_func in weights_table.items():
        fwd_requests[node_from] = {}
        for func, weights_x_node in weights_x_func.items():
            fwd_requests[node_from][func] = {}

            if sum(list(weights_x_node.values())) != 0:
                # For each request additional to max_rate, select a random node between available
                # with weights set as probability distribution
                choiches = np.random.choice(
                    list(weights_x_node.keys()),
                    size=(int(invoc_rate_table[node_from][func]) - int(max_rate_table[node_from][func])),
                    p=[w/100 for w in list(weights_x_node.values())]
                )

                for node_to in choiches:
                    # print("FROM: " + node_from)
                    # print("TO: " + node_to)
                    if node_to in list(fwd_requests[node_from][func].keys()):
                        fwd_requests[node_from][func][node_to] += 1
                    else:
                        fwd_requests[node_from][func][node_to] = 1
 
    # Fill forwarding table with missing values
    for node_from, weights_x_func in fwd_requests.items():
        for f in config_manager.FUNCTION_NAMES:
            if f not in list(weights_x_func.keys()):
                fwd_requests[node_from][f] = {}

    nodes_set = set(fwd_requests.keys())  # Set with node keys

    # Fill forwarding table with missing values (missing nodes)
    # Fill the table diagonal with max_rate if invoc_rate >= max_rate
    # or with invoc_rate if invoc_rate < max_rate
    # This means that along the main diagonal there will be the number of request
    # directly served by the node
    for node_from, weights_x_func in fwd_requests.items():
        for func, weights_x_node in weights_x_func.items():
            if func in config_manager.FUNCTION_NAMES:
                for node in nodes_set:
                    if node not in list(weights_x_node.keys()):
                        fwd_requests[node_from][func][node] = 0
                if invoc_rate_table[node_from][func] < max_rate_table[node_from][func]:
                    fwd_requests[node_from][func][node_from] = invoc_rate_table[node_from][func]
                else:
                    fwd_requests[node_from][func][node_from] = max_rate_table[node_from][func]

    # Utility prints
    # print("============= FORWARDING TABLE TABLE ==============")
    # print(fwd_requests)
    # print("============================================================")

    return fwd_requests


def create_tables(fwd_requests, invoc_rate, max_rate, minute, strategy_type):
    """
    Starting by forwarding requests create a table and export it in a CSV file
    Also invocation rate and max rate table are create and exported in the same 
    format
    """
    path = config_manager.SIMULATION_TABLES_OUTPUT_PATH.joinpath(strategy_type, "minute_" + str(minute))
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

    # Create dataframe for invoc_rates
    df_invoc = pd.DataFrame([], index=config_manager.FUNCTION_NAMES, columns=nodes_set)
    for node in invoc_rate:
        df_invoc[node] = [invoc_rate[node][f] for f in config_manager.FUNCTION_NAMES]
    df_invoc = df_invoc.T
    print("     > INVOC_RATE_TABLE")
    print(df_invoc)

    df_invoc.to_csv(path.joinpath("invoc_rates.csv"), sep='\t', encoding='utf-8')

    # Create dataframe for max_rates
    df_max_rates = pd.DataFrame([], index=config_manager.FUNCTION_NAMES, columns=nodes_set)
    for node in max_rate:
        df_max_rates[node] = [max_rate[node][f] for f in config_manager.FUNCTION_NAMES]

    df_max_rates = df_max_rates.T
    df_max_rates.to_csv(path.joinpath("max_rates.csv"), sep='\t', encoding='utf-8')
    print("     > MAX_RATE_TABLE")
    print(df_max_rates)


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


def simulation(nodes_number, config_file):
    """
    This function allow to simulate various strategies for workload distribution
    and use weights to distribuite the load across neighbours
    """
    dl = DataLoader()

    # Execution time dictionary
    execution_times = {}

    # Initialize execution time map for each strategy
    for s in config_manager.STRATEGIES:
        execution_times[s] = []

    for minute in range(0, config_manager.SIMULATION_MINUTES):  # 6 minutes
        # Dictionary that contains final json configuration
        final_config = {}

        # Dictionaries used for analysis
        simulation_weights_table = {}
        simulation_invoc_rate_table = {}        
        simulation_max_rate_table = {}

        # Forwarding requests dictionary
        fwd_requests = {}

        # Initialize maps for each strategy
        for s in config_manager.STRATEGIES:
            simulation_weights_table[s] = {}
            fwd_requests[s] = {}

        # Create global configuration file with info of all nodes
        for i in range(0, nodes_number):
            key = config_manager.NODE_KEY_PREFIX + str(i)
            final_config[key] = config_file[key]["exp_history"][minute]

            # Ask for a configuration to data loader module
            function_requests = []
            for func in final_config[key]["functions"]:
                if func["name"] in config_manager.FUNCTION_NAMES:
                    # This json objects has only "name" and "invoc_rates"
                    function_requests.append(
                        FunctionRequest(
                            func["name"],
                            config_file[key]["replicas"][func["name"]],
                            func["invoc_rate"]
                        )
                    )

            config_request = ConfigRequest(
                config_file[key]["node_type"],
                function_requests
            )

            print("--------------------- CONFIG REQUEST ---------------------")
            print("Query to database for configuration: {}".format(config_request))

            # Obtain metrics for this specific node configuration
            df_node, df_func = dl.get_metric_for_configuration(config_request)

            # Debug print on file
            #df_node.to_csv("df_node.csv")
            #df_func.to_csv("df_func.csv")

            print("----------------------------------------------------------")

            # Parse node metrics
            for _, metric in df_node[["MetricName", "AVG(Value)"]].T.to_dict().items():
                final_config[key][metric["MetricName"]] = metric["AVG(Value)"] / 100

            # Parse function metrics
            for func in final_config[key]["functions"]:
                if func["name"] in config_manager.FUNCTION_NAMES:
                    tmp_df = df_func[["MetricName", "AVG(Value)", "FunctionName", "MaxRate",
                                      "NumReplicas", "Margin", "State"]]
                    tmp_df = tmp_df[tmp_df["FunctionName"] == func["name"]]
                    for _, metric in tmp_df.T.to_dict().items():
                        func[metric["MetricName"]] = metric["AVG(Value)"]

                    func["service_count"] = tmp_df["NumReplicas"].unique().item(0)
                    func["margin"] = tmp_df["Margin"].unique().item(0)
                    func["state"] = tmp_df["State"].unique().item(0)
                    func["max_rate"] = tmp_df["MaxRate"].unique().item(0)

            # Create and fill invoc_rate and max_rate dictionaries with loaded values
            simulation_invoc_rate_table[key] = {}
            simulation_max_rate_table[key] = {}
            for func in final_config[key]["functions"]:
                if func["name"] in config_manager.FUNCTION_NAMES:
                    # Fill tables
                    simulation_invoc_rate_table[key][func["name"]] = func["invoc_rate"]
                    simulation_max_rate_table[key][func["name"]] = func["max_rate"]

        # Fill invoc_rate table with missing values
        for node, weights_x_func in simulation_invoc_rate_table.items():
            for f in config_manager.FUNCTION_NAMES:
                if f not in list(weights_x_func.keys()):
                    simulation_invoc_rate_table[node][f] = 0

        # Fill max_rate table with missing values
        for node, weights_x_func in simulation_max_rate_table.items():
            for f in config_manager.FUNCTION_NAMES:
                if f not in list(weights_x_func.keys()):
                    simulation_max_rate_table[node][f] = 0

        # Write configuration on json file for logging
        with open(config_manager.SIMULATION_COMPLETE_CONFIGURATION_OUTPUT_PATH.joinpath(
                  'config{}.json'.format(minute)), 'w', encoding='utf-8') as f:
            json.dump(final_config, f, ensure_ascii=False, indent=4)

        # Call agent loop for each config that has been previously built
        #
        # With last update this code is executed for each type of behaviour
        # (base, random and empirical) and for each agent in the network
        for id in range(0, nodes_number):
            key = config_manager.NODE_KEY_PREFIX + str(id)
            config_with_neigh = {}
            config_with_neigh[key] = final_config[key] # Add this node
            neighbours = config_file[key]["neighbours"]

            # Create configuration file with only neighbours
            for neighbour in neighbours:
                config_with_neigh[neighbour] = final_config[neighbour]

            #print(config_with_neigh)

            logger = get_logger(
                "agent" + str(id) + "_minute_" + str(minute),
                config_manager.SIMULATION_AGENT_LOGGING_BASE_PATH.joinpath("agent_" +
                str(id) + ".log"),
                logging.INFO
            )

            logger.info("\n")
            logger.info("-------- MINUTE {} --------".format(minute))

            # Execute agent loop for each strategy
            for s in config_manager.STRATEGIES:
                # Build correct strategy
                strategy = StrategyFactory.create_strategy(s, config_with_neigh)
                logger.info("   > STRATEGY: {} <".format(s))
                agent = Agent(
                    id,
                    logger,
                    strategy
                )
                #agent.disable_logging() # Disable logging for speed
                weights, execution_time = run_agent(agent)
                execution_times[s].append(execution_time)
                simulation_weights_table[s][key] = weights

        for s in config_manager.STRATEGIES:
            fwd_requests[s] = xfunc_request_table(
                simulation_max_rate_table, 
                simulation_invoc_rate_table,
                simulation_weights_table[s]
            )

        print("> START MINUTE {}".format(minute))

        for s in config_manager.STRATEGIES:
            # Create and export tables for three algorithms
            print(" > {}".format(s))
            create_tables(fwd_requests[s], simulation_invoc_rate_table, simulation_max_rate_table, minute, s)
            print("------------------------------------------------")

        print("> END MINUTE {}".format(minute))

    return {k: np.mean(times_for_algo) for k, times_for_algo in execution_times.items()}


def main(instance_file=""):
    if instance_file == "":
        instance_file = config_manager.OUTPUT_INSTANCE_JSON_FILE_PATH
        
    f = open(instance_file)
    config_file = json.load(f)
    simulation(config_file["nodes_number"], config_file)


# Call main program.
if __name__ == "__main__":
    main()
