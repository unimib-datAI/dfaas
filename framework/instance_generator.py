# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import os
import json
import networkx as nx
import random
import matplotlib.pyplot as plt
import numpy as np
import math
from cli.cli import get_args
from itertools import combinations, groupby
from configuration.config_manager import ConfigManager
import pandas as pd

config_manager = ConfigManager()

def generate_configs(nodes_number, overloaded_max_percentage, seed):
    net_gen = np.random.default_rng(seed)
    load_gen = np.random.default_rng(seed)

    n_max_overload = round(overloaded_max_percentage * nodes_number)
    actual_overload = 0
    # Fetching node types and their probability distribution from the config_manager
    nodes_type = config_manager.NODES_TYPES
    nodes_type_prob_dist = config_manager.NODES_TYPE_PROBABILITY_DISTRIBUTION

    # Randomly choosing node types based on the provided probability distribution
    choices = net_gen.choice(nodes_type, nodes_number, p=nodes_type_prob_dist)

    # List to store the generated configurations
    configurations = []

    # Iterating over each chosen node type
    for choice in choices:
        # random.seed(seed)
        # np.random.seed(seed)
        # Configuration dictionary for the current node
        node_config = {}
        node_config["node_type"] = choice
        node_config["load"] = []

        # Randomly determining the number of groups for the current node
        # Each node should have at least two groups in order to guarantee the overload in at least the peak istant
        node_groups_number = net_gen.integers(1,4)

        # Randomly selecting groups from the available groups in config_manager
        groups = net_gen.choice(list(config_manager.GROUPS.keys()), node_groups_number, replace=False)

        # Dictionary to store functions for each group
        functions_groups = {}
        for group in groups:
            # Randomly selecting a subset of functions for each group
            num_elements_to_choose = net_gen.integers(1, len(config_manager.GROUPS[group]) + 1)
            # TODO: puoi usare il choice del net_gen al posto del sample
            functions_groups[group] = net_gen.choice(config_manager.GROUPS[group], num_elements_to_choose, replace=False)

        max_rates_o = {"HEAVY": {"LOW_USAGE": 400, "HIGH_USAGE": 90, "MEDIUM_USAGE": 200},
                        "MID": {"LOW_USAGE": 570, "HIGH_USAGE": 80, "MEDIUM_USAGE": 200},
                       "LIGHT": {"LOW_USAGE": 330, "HIGH_USAGE": 40, "MEDIUM_USAGE": 180}}

        # Create a new dictionary with values as 15% of the original values
        max_rates_u = {key1: {key2: value * 0.15 for key2, value in inner_dict.items()} for key1, inner_dict in max_rates_o.items()}


        overloaded_node = 1 if load_gen.random() <= overloaded_max_percentage else 0

        if overloaded_node == 1 and actual_overload < n_max_overload:
            max_rates = max_rates_o
            actual_overload += 1
        else:
            max_rates = max_rates_u

        # For each time instant in the simulation
        for instant in range(0, config_manager.SIMULATION_MINUTES):

            # Dictionary to store configurations for each group
            config_group = {}

            # For each group in the node
            for group in groups:
                # List to store function configurations for the current group
                group_list = []

                # Triangle generation increasing until the middle simulation instant and decreasing after that
                maximum_rate_instant = math.ceil(config_manager.SIMULATION_MINUTES/2)

                if instant < maximum_rate_instant:
                    group_load = load_gen.integers(math.floor(instant * (max_rates[choice][group] / maximum_rate_instant)), math.ceil((instant + 1) * (max_rates[choice][group] / maximum_rate_instant)), endpoint=True)
                else:
                    top_instant = config_manager.SIMULATION_MINUTES
                    group_load = load_gen.integers(math.floor((top_instant - (instant + 1)) * (max_rates[choice][group] / maximum_rate_instant)), math.ceil((top_instant - instant) * (max_rates[choice][group] / maximum_rate_instant)), endpoint=True)

                group_load = int(group_load)

                # Generate random load values for each function
                loads = [load_gen.integers(0, group_load, endpoint=True) for _ in range(len(functions_groups[group]))]

                # Normalize the loads to sum up to total_load
                load_sum = sum(loads)

                if load_sum == 0:
                    normalized_loads = [0] * len(functions_groups[group])
                else:
                    normalized_loads = [load * group_load // load_sum for load in loads]

                # Adjust for rounding errors
                remaining_load = group_load - sum(normalized_loads)
                normalized_loads[0] += remaining_load

                # Creates the config for each function
                for i in range(0, len(functions_groups[group])):
                    function_config = {
                        "function_name": functions_groups[group][i],
                        "function_rate": int(normalized_loads[i])
                    }
                    group_list.append(function_config)

                # Adding a new level of nesting for each group
                config_group[group] = {
                    "functions": group_list,
                    "total_rate": group_load
                }

            # Adding group configurations to the node's load
            node_config["load"].append(config_group)

        # Adding node configuration to the list of configurations
        #plot_node_load(node_config, groups)
        configurations.append(node_config)
        

    # Printing the generated configurations (for debugging purposes)
    #print(configurations)

    return configurations



def gnp_random_connected_graph(n, p):
    """
    Generates a random undirected graph, similarly to an Erdős-Rényi
    graph, but enforcing that the resulting graph is connected

    See: https://stackoverflow.com/questions/61958360/how-to-create-random-graph-where-each-node-has-at-least-1-edge-using-networkx
    """
    edges = combinations([el[0] for el in n], 2)
    G = nx.Graph()
    G.add_nodes_from(n)
    if p <= 0:
        return G
    if p >= 1:
        return nx.complete_graph(n, create_using=G)
    for _, node_edges in groupby(edges, key=lambda x: x[0]):
        node_edges = list(node_edges)
        random_edge = random.choice(node_edges)
        G.add_edge(*random_edge)
        for e in node_edges:
            if random.random() < p:
                G.add_edge(*e)
    return G

def plot_graph(G):
    """
    Plot graph and export on file
    """
    plt.figure(figsize=(10, 6))
    nx.draw(G, node_color='lightblue',
            with_labels=True,
            node_size=2000)
    path = config_manager.OUTPUT_INSTANCE_PATH
    if not os.path.exists(path):
        os.makedirs(path)
    plt.savefig(config_manager.OUTPUT_INSTANCE_GRAPH_FILE_PATH)

def dict_key_substitution(data, old, new):
    """
    Utility function used to substitute dictionary key
    """
    data[new] = data[old]
    del data[old]

def plot_node_load(node_config, groups):
    # Plot configurations
    plt.figure(figsize=(20, 10))
    plt.title("Rate for each group in a {} node during {} minutes of experiment".format(node_config["node_type"], config_manager.SIMULATION_MINUTES))
    plt.xlabel("Minute")
    plt.ylabel("Requests per second")

    # Initialize lists to store data
    low_usage = []
    medium_usage = []
    high_usage = []

    # Extract data into separate lists
    for entry in node_config['load']:
        low_total_rate = entry.get('LOW_USAGE', {}).get('total_rate', None)
        medium_total_rate = entry.get('MEDIUM_USAGE', {}).get('total_rate', None)
        high_total_rate = entry.get('HIGH_USAGE', {}).get('total_rate', None)
        
        low_usage.append(low_total_rate)
        medium_usage.append(medium_total_rate)
        high_usage.append(high_total_rate)

    # Create DataFrame
    df = pd.DataFrame({
        'LOW_USAGE': low_usage,
        'MEDIUM_USAGE': medium_usage,
        'HIGH_USAGE': high_usage
    })
    
    # Define line styles and colors
    line_styles = ['-', '--', '-.', ':']
    colors = ['blue', 'green', 'red', 'orange']
     # Plot each column with a different line style and color
    for i, column in enumerate(df.columns):
        linestyle = line_styles[i % len(line_styles)]
        color = colors[i % len(colors)]
        plt.plot(df.index, df[column], label="r/s for {}".format(column), linestyle=linestyle, color=color)

    # Plot configurations
    plt.legend(loc="upper right")
    plt.grid()
    
    plt.savefig(config_manager.OUTPUT_INSTANCE_PATH.joinpath('temp.png'))

def create_node_representation(key, start_config, G):
    """
    Create node json representation in new format of instance file
    """
    node_config = {}

    node_config["node_type"] = start_config["node_type"]
    node_config["neighbours"] = list(G[key].keys())
    node_config["load"] = start_config["load"]

    return node_config

def build_output_json(seed, nodes_num, edge_prob, overloaded_max_percentage, G):
    """
    Function used to build output json file that represents the instance
    """
    instance = {}
    instance["seed"] = seed
    instance["nodes_number"] = nodes_num
    instance["edge_prob"] = edge_prob
    instance["overloaded_perc"] = overloaded_max_percentage * 100

    # Iterate over all graph nodes
    for node in G.nodes(data=True):
        key, config = node[0], node[1]
        instance[key] = create_node_representation(key, config["config"], G)

    return instance

def export_instance_file(instance):
    with open(config_manager.OUTPUT_INSTANCE_JSON_FILE_PATH, 'w', encoding='utf-8') as f:
        json.dump(instance, f, ensure_ascii=False, indent=4)

def main():
    # Get args passed as params
    kargs = get_args()
    print(kargs)
    nodes_num = kargs["nodesnum"]
    seed = kargs["seed"]
    probability = kargs["edgeprob"]
    overloaded_max_percentage = kargs["overloaded"]

    # Assign default values to params if not defined
    if nodes_num == -1:
        nodes_num = 10
    if seed == -1:
        seed = np.random.randint(4097)
    if probability == -1.0:
        probability = 0.1
    if overloaded_max_percentage == -1:
        overloaded_max_percentage = 60

    overloaded_max_percentage *= 0.01

    #print(*(nodes_num, seed, probability, max_rates))
    # raise(Exception)
    random.seed(seed)
    np.random.seed(seed)

    # Load "nodes_num" configuration file
    generated_configs = generate_configs(nodes_num, overloaded_max_percentage, seed)

    # print(loaded_files)
    # Create a random graph with "nodes_num" nodes
    # Nodes are tuple of (node_id, json_config)
    # json_config is used as a node property
    nodes = []
    for i, config in zip(range(0, nodes_num), generated_configs):
        key = config_manager.NODE_KEY_PREFIX + str(i)
        nodes.append((key, {"config": config}))
    #print(nodes)
    G = gnp_random_connected_graph(nodes, probability)

    # Export an image of graph
    plot_graph(G)

    # Print neighbor for each node
    # Utility print
    print(" > Neighborhood:")
    for n in nodes:
        print("     > Node {} neighbors: {}".format(n[0], G[n[0]]))

    # Build instance file (JSON file) with all information
    instance_json = build_output_json(seed, nodes_num, probability, overloaded_max_percentage, G)

    # Export instance file
    export_instance_file(instance_json)

# Call the main program.
if __name__ == "__main__":
    main()
