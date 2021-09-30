import os
import json
import networkx as nx
import random
import matplotlib.pyplot as plt
import numpy as np
from cli import get_args
from os import listdir
from os.path import isfile, join
from itertools import combinations, groupby
from config_manager import ConfigManager

config_manager = ConfigManager()

def gather_configurations():
    """
    Gather config file informations from data directory
    """
    nodes_type = config_manager.NODES_TYPE
    data_dir = config_manager.DATA_DIR

    exp_files_path = {}
    for node_type in nodes_type:
        exp_files_path[node_type] = []

        for path, subdirs, files in os.walk(os.path.join(data_dir, node_type)):
            for name in files:
                exp_files_path[node_type].append(os.path.join(path, name))

    return exp_files_path

def load_configurations(nodes_number):
    nodes_type = config_manager.NODES_TYPE
    nodes_type_prib_dist = config_manager.NODES_TYPE_PROBABILITY_DISTRIBUTION
    config_files = gather_configurations()

    # For each node pick a random file
    configurations = []

    # Not used a uniform distribution because is more likely to have a
    # high number of node of type 1, with minor specs due to gerarchic
    # cloud infrastructure
    choiches = np.random.choice(nodes_type, nodes_number, p=nodes_type_prib_dist)
    for choice in choiches:
        config = np.random.choice(config_files[choice])
        configurations.append(config)
    #print(configurations)

    # Mock loaded files [TO BE REMOVED]
    # configurations = ["exp-comparison/case3/node1.json",
    #                   "exp-comparison/case3/node2.json",
    #                   "exp-comparison/case3/node3.json"]
    # print(configurations)

    # Load selected files
    loaded_json = []
    for config in configurations:
        f = open(config)
        json_doc = json.load(f)  # Return json file as a dictionary
        loaded_json.append(json_doc)
    
    return loaded_json


def gnp_random_connected_graph(n, p):
    """
    Generates a random undirected graph, similarly to an Erdős-Rényi 
    graph, but enforcing that the resulting graph is conneted
    
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
    plt.savefig(config_manager.OUTPUT_INSTANCE_GRAPH_FILE_PATH)

def dict_key_substitution(data, old, new):
    """
    Utility function used to substitute dictionary key
    """
    data[new] = data[old]
    del data[old]

def build_output_json(seed, nodes_num, G):
    """
    Function used to build output json file that represent the instance
    """
    instance = {}
    instance["seed"] = seed
    instance["nodes_number"] = nodes_num

    # Iterate over all graph nodes
    for node in G.nodes(data=True):
        key, config = node[0], node[1]
        instance[key] = config["config"]["input"]

        # Keys transformations
        dict_key_substitution(instance[key], "node", "node_type")
        
        # Key specific for three functions: funca, qrcode, ocr
        # dict_key_substitution(instance[key], "funcb_num", "qrcode_num")
        # dict_key_substitution(instance[key], "funcc_num", "ocr_num")
        # dict_key_substitution(instance[key], "funcb_wl", "qrcode_wl")
        # dict_key_substitution(instance[key], "funcc_wl", "ocr_wl")

        instance[key]["neighbours"] = list(G[key].keys())
        instance[key]["exp_history"] = config["config"]["output"]

    return instance

def export_instance_file(instance):
    with open(config_manager.OUTPUT_INSTANCE_JSON_FILE_PATH, 'w', encoding='utf-8') as f:
        json.dump(instance, f, ensure_ascii=False, indent=4)

def main():
    # Get args passed as params
    kargs = get_args()
    #print(kargs)
    nodes_num = kargs["nodesnum"]
    seed = kargs["seed"]
    probability = kargs["edgeprob"]

    # Load "nodes_num" configuration file
    loaded_files = load_configurations(nodes_num)

    # Create a random graph with "nodes_num" nodes
    # Nodes are touple of (node_id, json_config)
    # json_config is used as a node property
    nodes = []
    for i, config in zip(range(0, nodes_num), loaded_files):
        key = "node_" + str(i)
        nodes.append((key, {"config": config}))

    random.seed(seed)
    np.random.seed(seed)

    G = gnp_random_connected_graph(nodes, probability)

    # Try to export an image of graph
    plot_graph(G)

    # Print neghbour foreach node
    # Utility print
    print(" > Neighbourhood:")
    for n in nodes:
        print("     > Node {} neighbours: {}".format(n[0], G[n[0]]))

    # Build instance file (JSON file) with all informations
    instance_json = build_output_json(seed, nodes_num, G)

    # Export instance file
    export_instance_file(instance_json)

# Call main program.
if __name__ == "__main__":
    main()
