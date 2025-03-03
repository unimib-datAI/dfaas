# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import pandas as pd
import os
import math
import numpy as np
import matplotlib.pyplot as plt
from configuration.config_manager import ConfigManager
from model.model_proxy import ModelProxy
from utils.utils import flatten
import json
from cli.cli import get_analyzer_and_simulator_args

config_manager = ConfigManager()
model_manager = ModelProxy()

def calculate_rates_globally(function_rate_x_node, df_presence, config_file, strategy_name):
    # It contains the number of requests rejected in this instant for each function
    func_reject_num = {}

    # It contains the success rate of this instant for each function
    success_rate = {}

    # It contains the reject rate of this instant for each function
    reject_rate = {}

    # Reject the requests of the functions that are not deployed in the node
    for f in config_manager.FUNCTION_NAMES:
        func_reject_num[f] = 0
        for index, row in df_presence.iterrows():
            if df_presence.loc[index, f] == 0:
                func_reject_num[f] += function_rate_x_node.loc[index, f]

    # Average consumption of the node in the current istant
    node_avg_power_consumption = 0
    # Max consumption of the nodes in the current istant
    max_power_consumption = 0
    # Overloaded nodes counter
    overloaded_nodes = 0
    
    # Iterate over all the node
    for index, row in df_presence.iterrows():
        # Reset the rate of the functions that are not deployed on the node
        rate_only_present_functions = function_rate_x_node.loc[index] * df_presence.loc[index]
        # Obtains data in group format
        features_data = model_manager.transform_functions_in_groups(rate_only_present_functions)

        # Add the information about the node type
        node_type = config_file[index]["node_type"]
        features_data["node_type"] = config_manager.NODES_TYPES_IN_MODELS[node_type]

        node_predictions = model_manager.get_node_predictions(features_data)
        node_power_consumption = node_predictions["power_usage_node"].iloc[0]
        overload = node_predictions["overloaded_node"].iloc[0]

        if overload == 1:
            overloaded_nodes += 1

        # Percentage of load to discard
        load_discard_percentage = 0.05
        while overload == 1 or (strategy_name == "power_saving_strategy" and node_power_consumption > config_manager.MAX_POWER_USAGE[node_type]) :
            # Calculate the the percentage value for each element
            percentage_values = [round(element * load_discard_percentage) for element in rate_only_present_functions]

            # If all calculated values are 0, then increment the percentage 
            while all(value == 0 for value in percentage_values):
                load_discard_percentage += 0.01
                
                # Calculate the new values
                percentage_values = [round(element * load_discard_percentage) for element in rate_only_present_functions]                

            # Remove a percentage of the requests for each function > 0 
            for i in range(0, len(rate_only_present_functions)):
                if rate_only_present_functions[i] > 0:
                    # Calculate the rate to remove for the selected function
                    rate_to_remove = percentage_values[i]
                    
                    # Remove the rate of the function from the node
                    rate_only_present_functions[i] -= rate_to_remove

                    # Count the removed rate in the total reject req
                    func_reject_num[rate_only_present_functions.index[i]] += rate_to_remove
            
            # Return features data in the correct from to be predicted
            features_data = model_manager.transform_functions_in_groups(rate_only_present_functions)
 
            # Add the information about the node type
            features_data["node_type"] = config_manager.NODES_TYPES_IN_MODELS[config_file[index]["node_type"]]
            
            node_predictions = model_manager.get_node_predictions(features_data)

            node_power_consumption = node_predictions["power_usage_node"].iloc[0]
            overload = node_predictions["overloaded_node"].iloc[0]

        if node_power_consumption > max_power_consumption:
            max_power_consumption = node_power_consumption
        
        # Add the node consumption to the total
        node_avg_power_consumption += node_power_consumption


    # Get the average consumption for node
    node_avg_power_consumption /= len(df_presence)
    
    # Calculate the success and the reject rate for each function
    total_invoc_rate_x_function = function_rate_x_node.sum(axis = 0)
    for func in func_reject_num:
        success_number = total_invoc_rate_x_function[func] - func_reject_num[func]
        success_rate[func] = (success_number / total_invoc_rate_x_function[func]) if total_invoc_rate_x_function[func] > 0  else 1.0
        reject_rate[func] = 1.0 - success_rate[func]

    return success_rate, reject_rate, func_reject_num, node_avg_power_consumption, max_power_consumption, overloaded_nodes


def export_for_minute_overloaded_nodes(overloaded_for_algo):
    """
    Export plot that represent success rate during all minutes of experiment
    """
    # Plot configurations
    plt.figure(figsize=(20, 10))
    plt.title("Overloaded nodes over all the minutes of experiment")
    plt.xlabel("Minute")
    plt.ylabel("N° of overloaded node")

    df = pd.DataFrame(data=overloaded_for_algo, index=[i for i in range(0, config_manager.SIMULATION_MINUTES)])
    for column in df.columns:
        plt.plot(df.index, df[column], label="Overloaded nodes for {}".format(column))

    # Plot configurations
    plt.legend(loc="lower left")
    plt.grid()
    
    plt.savefig(create_path_if_not_exists(config_manager.ANALYZER_OUTPUT_PATH).joinpath("comparison_nodes.png"))


def export_for_minute_rates(func, rates):
    """
    Export plot that represent success rate during all minutes of experiment
    """
    # Plot configurations
    plt.figure(figsize=(20, 10))
    plt.title("Success rate for function {} during {} minutes of experiment".format(func, config_manager.SIMULATION_MINUTES))
    plt.xlabel("Minute")
    plt.ylabel("Success rate")

    df = pd.DataFrame(data=rates, index=[i for i in range(0, config_manager.SIMULATION_MINUTES)])
    #print(df)

    # Define line styles and colors
    line_styles = ['-', '--', '-.', ':']
    colors = ['blue', 'green', 'red', 'orange']

    # Plot each column with a different line style and color
    for i, column in enumerate(df.columns):
        linestyle = line_styles[i % len(line_styles)]
        color = colors[i % len(colors)]
        plt.plot(df.index, df[column], label="Success rate for {}".format(column), linestyle=linestyle, color=color)

    # Plot configurations
    plt.legend(loc="lower left")
    plt.grid()
    
    plt.savefig(create_path_if_not_exists(config_manager.ANALYZER_OUTPUT_PATH).joinpath("comparison_{}.png".format(func)))


def export_index_comparison_table(df):
    """
    Export index comparison table of different strategies as CSV file
    """
    df.to_csv(config_manager.INDEX_COMPARISON_FILE, sep=',', encoding='utf-8')

# Create the specific path if not present in the machine    
def create_path_if_not_exists(path):
    if not os.path.exists(path):
        os.makedirs(path)
    return path

def main():
    kargs = get_analyzer_and_simulator_args()
    model_manager.set_model_type(kargs["modeltype"])
    rates_for_algo = {}
    overloaded_for_algo = {}
    index_comparison = pd.DataFrame(index=config_manager.INDEX_TO_COMPARE)

    # Read the instance file
    instance_file = config_manager.OUTPUT_INSTANCE_JSON_FILE_PATH
    with open(instance_file) as f:
        config_file = json.load(f)

    # Initialize DataFrame
    df_presence = pd.DataFrame(0, index=config_manager.FUNCTION_NAMES, columns=[])

    # Populate DataFrame
    for node_key, node_value in config_file.items():
        if "node_" in node_key:
            node_functions = [
                function_info["function_name"] 
                for k, v in node_value["load"][0].items() 
                for function_info in v["functions"]
            ]
            df_presence[node_key] = df_presence.index.isin(node_functions).astype(int)

    df_presence = df_presence.sort_index(axis=1).T

    # Used only for initialization
    for func in config_manager.FUNCTION_NAMES:
        rates_for_algo[func] = {}

    # For each strategy type, for each minute and for each function read data exported
    # by the simulation and use them to calculate rates and indexes for comparison
    for algo in config_manager.STRATEGIES:
        x_func_success_rate = {}
        x_func_reject_rate = {}
        x_func_reject_num = {}
        x_node_power_consumption = []
        x_node_max_power_consumption = []
        x_overloaded_nodes_counter = []

        # Initialize dictionary of rates for all functions
        for func in config_manager.FUNCTION_NAMES:
            x_func_success_rate[func] = []
            x_func_reject_rate[func] = []
            x_func_reject_num[func] = []

        print("-------------------------- ALGO {} --------------------------".format(algo))

        # Create path for recover tables        
        base_path = config_manager.SIMULATION_TABLES_OUTPUT_PATH.joinpath(algo)

        for minute in range(0, config_manager.SIMULATION_MINUTES):
            print("MINUTE {}".format(minute))
            print(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")

            # Complete path for load tables
            path = base_path.joinpath("minute_" + str(minute))

            # Table which contains the rates after the exchanges of each function that each node has
            function_rate_x_node = pd.DataFrame()
            
            # For each minute and foreach function load dataframe
            for func in config_manager.FUNCTION_NAMES:
                df = pd.read_csv(path.joinpath(func + ".csv"), delimiter='\t', header=0, index_col=0)
                function_rate_x_node[func] = df.sum(axis=0)
 
            sr, rr, rn, an, mc, on = calculate_rates_globally(function_rate_x_node, df_presence, config_file, algo)
            x_node_power_consumption.append(an)
            x_node_max_power_consumption.append(mc)
            x_overloaded_nodes_counter.append(on)


            for func in sr:
                x_func_success_rate[func].append(sr[func])
                x_func_reject_rate[func].append(rr[func])
                x_func_reject_num[func].append(rn[func])
                rates_for_algo[func][algo] = x_func_success_rate[func]

            print("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")

        overloaded_for_algo[algo] = x_overloaded_nodes_counter
        print("STATS FOR ALGO {}".format(algo))
        # Metrics prints

        ##### SUCCESS RATES METRICS #####
        # Mean success rate
        mean_success_rate = np.mean([np.mean(srates) for k, srates in x_func_success_rate.items()]) * 100
        print("     > Mean success rate: {:0.2f}%".format(
            mean_success_rate
        ))

        # Mean node consumption
        mean_node_consumption = sum(x_node_power_consumption) / len(x_node_power_consumption)        
        print("     > Mean node power consumption: {:0.2f}%".format(
            mean_node_consumption
        ))

        # Max node consumption
        max_node_consumption = max(x_node_max_power_consumption)    
        print("     > Max node power consumption: {:0.2f}%".format(
            max_node_consumption
        ))

        # Success rate variance
        # flat_list = [i * 100 for i in flatten(list(x_func_success_rate.values()))]
        # success_rate_variance = np.var(flat_list)
        # print("     > Success rate variance: {:0.2f}".format(success_rate_variance))

        # Success rate median
        flat_list = flatten(list(x_func_success_rate.values()))
        success_rate_median = np.median(flat_list) * 100
        print("     > Success rate median: {:0.2f}%".format(success_rate_median))

        # Success rate percentile
        # flat_list = flatten(list(x_func_success_rate.values()))
        # success_rate_percentile = np.percentile(flat_list, config_manager.ANALYSIS_PERCENTILE) * 100
        # print("     > Success rate {}% percentile: {:0.2f}%".format(
        #         config_manager.ANALYSIS_PERCENTILE,
        #         success_rate_percentile
        #     )
        # )

        ##### SUCCESS RATES (STRESS PERIOD) METRICS #####
        # Mean success rate calculated during high traffic period
        mid_instant = math.floor(config_manager.SIMULATION_MINUTES / 2)
        low = mid_instant - 1
        high = mid_instant + 1
        mean_success_rate_stress_period = np.mean([np.mean(srates[low:high]) for k, srates in x_func_success_rate.items()]) * 100
        print("         > Mean success rate during stress period: {:0.2f}%".format(
            mean_success_rate_stress_period
        ))

        # Success rate variance (stress period)
        # flat_list = [i * 100 for i in flatten([item[1:6] for item in list(x_func_success_rate.values())])]
        # success_rate_stress_period_variance = np.var(flat_list)
        # print("         > Success rate variance during stress period (from minute 1 to 5): {:0.2f}"
        #     .format(success_rate_stress_period_variance))

        # Success rate median (stress period)
        flat_list = flatten([item[low:high] for item in list(x_func_success_rate.values())])
        success_rate_stress_period_median = np.median(flat_list) * 100
        print("         > Success rate median during stress period: {:0.2f}%"
            .format(success_rate_stress_period_median))

        # Success rate percentile (stress period)
        # flat_list = flatten([item[1:6] for item in list(x_func_success_rate.values())])
        # success_rate_stress_period_percentile = np.percentile(flat_list, config_manager.ANALYSIS_PERCENTILE) * 100
        # print("         > Success rate {}% percentile during stress period (from minute 1 to 5): {:0.2f}%"
        #     .format(
        #         config_manager.ANALYSIS_PERCENTILE,
        #         success_rate_stress_period_percentile
        #     )
        # )

        ##### REJECT RATES METRICS #####
        # Total rejected requests num calculated for each algorithm across minutes
        total_reject_requests = np.sum([np.sum(rejnums) for k, rejnums in x_func_reject_num.items()])
        print("     > Total rejected requests: {} req".format(
            total_reject_requests
        ))

        # Reject number variance
        # flat_list = flatten(list(x_func_reject_num.values()))
        # reject_number_variance = np.var(flat_list)
        # print("     > Reject num variance: {:0.2f}".format(reject_number_variance))

        # Reject number median
        # flat_list = flatten(list(x_func_reject_num.values()))
        # reject_number_median = np.median(flat_list)
        # print("     > Reject num median: {:0.2f}".format(reject_number_median))

        # Reject number percentile
        # flat_list = flatten(list(x_func_reject_num.values()))
        # reject_number_percentile = np.percentile(flat_list, config_manager.ANALYSIS_PERCENTILE)
        # print("     > Reject num {}% percentile: {:0.2f}".format(
        #         config_manager.ANALYSIS_PERCENTILE,
        #         reject_number_percentile
        #     )
        # )

        print("----------------------------------------------------------------------------")

        index_comparison[algo] = [
            mean_success_rate,
            success_rate_median,
            mean_success_rate_stress_period,
            success_rate_stress_period_median,
            mean_node_consumption,
            max_node_consumption,
            total_reject_requests
        ]

    # Export print for comparison
    for func in config_manager.FUNCTION_NAMES:
        export_for_minute_rates(func, rates_for_algo[func])

    # Creates plot of overloaded nodes
    export_for_minute_overloaded_nodes(overloaded_for_algo)

    # Export index comparison table
    print("> INDEX COMPARISON TABLE")
    print(index_comparison.T)
    export_index_comparison_table(index_comparison.T)


# Call main program.
if __name__ == "__main__":
    main()
