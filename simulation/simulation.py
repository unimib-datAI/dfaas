import logging
import time
import os
import json
import pandas as pd
import matplotlib.pyplot as plt
import numpy as np
from agent import Agent
from os import listdir
from os.path import isfile, join
from behaviour.empirical_strategy import EmpiricalStrategy

# Get a specific logger with passed configurations
def get_logger(name, log_file, level=logging.INFO):
    handler = logging.FileHandler(log_file)
    logger = logging.getLogger(name)
    logger.setLevel(level)
    logger.addHandler(handler)

    return logger

data_dir = "../data/"
nodes = ["node1", "node2", "node3"]

def gather_configurations():
    node1, node2, node3 = [], [], []
    for path, subdirs, files in os.walk(os.path.join(data_dir, nodes[0])):
        for name in files:
            node1.append(os.path.join(path, name))
    #print(node1)

    for path, subdirs, files in os.walk(os.path.join(data_dir, nodes[1])):
        for name in files:
            node2.append(os.path.join(path, name))
    #print(node2)

    for path, subdirs, files in os.walk(os.path.join(data_dir, nodes[2])):
        for name in files:
            node3.append(os.path.join(path, name))
    #print(node3)
    
    return node1, node2, node3

def simulation(nodes_number, node1_config, node2_config, node3_config):
    # 1) For each node pick a random file
    configurations = []
    # Not used a uniform distribution because is more likely to have a
    # high number of node1 nodes, with minor specs due to gerarchic
    # cloud infrastructure
    choiches = np.random.choice(nodes, nodes_number, p=[0.5, 0.3, 0.2])
    for choice in choiches:
        if choice == "node1":
            config = np.random.choice(node1_config)
        elif choice == "node2":
            config = np.random.choice(node2_config)
        else:
            config = np.random.choice(node3_config)
        configurations.append(config)
    #print(configurations)

    # 2) Load selected files
    loaded_json = []
    for config in configurations:
        f = open(config)
        json_doc = json.load(f)  # Return json file as a dictionary
        loaded_json.append(json_doc)
    
    # 3) For each minute of simulation build a configuration file
    # composed by "num_nodes" entries. Each entry contains a node's
    # situation, with metric gathered during experiments
    execution_times = []
    for minute in range(0, 7): # 6 minutes
        final_config = {}
        for i, f in zip(range(0, nodes_number), loaded_json):
            key = "node_" + str(i)
            f['output'][minute]['node'] = f['input']['node']
            final_config[key] = f['output'][minute]     
        #print(final_config)

        # Write configuration on file
        with open('config{}.json'.format(minute), 'w', encoding='utf-8') as f:
            json.dump(final_config, f, ensure_ascii=False, indent=4)

        # 4) Call agent loop for each config that has been previously built
        a = Agent(EmpiricalStrategy(0, "", get_logger("agent" + str(minute), "minute_" + str(minute) + ".log"), False, final_config))
        
        # time.perf_counter() returns elapsed time in seconds
        # It is the best way to measure performance
        # See: https://www.geeksforgeeks.org/time-perf_counter-function-in-python/
        start = time.perf_counter()
        a.run()
        end = time.perf_counter()
        execution = end - start
        
        execution_times.append(execution)
        
    return np.mean(execution_times)

def simulation_with_graphs():
    exp_times = []
    for i in range(5, 105, 5):
        repaeted_simulation_times = []
        for _ in range(0, 20):
            sim_time = simulation(i, node1_config, node2_config, node3_config)
            repaeted_simulation_times.append(sim_time)
        mean = np.mean(repaeted_simulation_times)
        exp_times.append(mean)
        print("Mean execution time over 10 repeated experiments with {} nodes is: {}".format(i, mean))

    print(exp_times)

    df = pd.DataFrame()

    df["experiment"] = range(5, 105, 5)
    df["time"] = exp_times

    # Plot configurations
    plt.figure(figsize=(20, 10))
    plt.title(
        "Agent execution time in function of the p2p net size or the number of neighbours")
    plt.xlabel("Experiment file")
    plt.ylabel("MAPE loop -- execution time (seconds)")

    plt.plot(df["experiment"], df["time"],
            label="Experiment with differents agents num")

    # Plot configurations
    plt.legend(loc="upper left")
    plt.grid()

    plt.savefig("comparison.png")

node1_config, node2_config, node3_config = gather_configurations()
#simulation_with_graphs()
simulation(4, node1_config, node2_config, node3_config)
