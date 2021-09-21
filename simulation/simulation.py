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
from behaviour.random_strategy import RandomStrategy
from behaviour.base_strategy import BaseStrategy

# Get a specific logger with passed configurations
def get_logger(name, log_file, level=logging.INFO):
    handler = logging.FileHandler(log_file)
    logger = logging.getLogger(name)
    logger.setLevel(level)
    logger.addHandler(handler)

    return logger

data_dir = "../data/"
nodes = ["node1", "node2", "node3"]
function_names = ["funca", "qrcode", "ocr"]

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

def xfunc_request_table(max_rate_table, invoc_rate_table, weights_table):
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
                    size=(invoc_rate_table[node_from][func] - max_rate_table[node_from][func]), 
                    p=[w/100 for w in list(weights_x_node.values())]
                )

                for node_to in choiches:
                    # print("FROM: " + node_from)
                    # print("TO: " + node_to)
                    if node_to in list(fwd_requests[node_from][func].keys()):
                        fwd_requests[node_from][func][node_to] += 1
                    else:
                        fwd_requests[node_from][func][node_to] = 1

            #for node_to, w in weights_x_node.items():
                # Note: this is correct but when number of agents increase
                # this is not stable (es: more forwarded requests of received ones)
            #    fwd_requests[node_from][func][node_to] = int(round((w * \
            #        (invoc_rate_table[node_from][func] - max_rate_table[node_from][func])) / 100))
            #fwd_requests[node_from][func][node_from] = invoc_rate_table[node_from][func] # Fixed at line [84-87]

    # print("============= [BEFORE] FORWARDING TABLE TABLE ==============")
    # print(fwd_requests)
    # print("============================================================")
 
    # Fill with remianing values   
    for node_from, weights_x_func in fwd_requests.items():
        for f in function_names:
            if f not in list(weights_x_func.keys()):
                fwd_requests[node_from][f] = {}

    nodes_set = set(fwd_requests.keys())

    # Fill with remianing values
    for node_from, weights_x_func in fwd_requests.items():
        for func, weights_x_node in weights_x_func.items():
            for node in nodes_set:
                if node not in list(weights_x_node.keys()):
                    fwd_requests[node_from][func][node] = 0
            if invoc_rate_table[node_from][func] < max_rate_table[node_from][func]:
                fwd_requests[node_from][func][node_from] = invoc_rate_table[node_from][func]
            else:
                fwd_requests[node_from][func][node_from] = max_rate_table[node_from][func]
                    
    # print("============= [AFTER] FORWARDING TABLE TABLE ==============")
    # print(fwd_requests)
    # print("============================================================")
    
    #check_table_validity(fwd_requests, invoc_rate_table, weights_table, max_rate_table)
    return fwd_requests

def check_table_validity(table, invoc_rate_table, weights_table, max_rate_table):
    for node_from, functions in table.items():
        for func, fwd_requests in functions.items():
            s = sum(fwd_requests.values())
            if s == invoc_rate_table[node_from][func]:
                print("Sum for func {} from {} is equal to {} = invoc rate of {} : CHECK OK".format(
                    func, node_from, s, invoc_rate_table[node_from][func]))
            elif s < invoc_rate_table[node_from][func] and max_rate_table[node_from][func] <= invoc_rate_table[node_from][func]:
                print("Sum for func {} from {} is NOT equal to {} = invoc rate of {} but is OVERLOADED: CHECK OK".format(
                    func, node_from, s, invoc_rate_table[node_from][func]))
            else:
                print("Sum for func {} from {} is NOT equal to {} = invoc rate of {} and UNVERLOADED: CHECK FAILED".format(
                    func, node_from, s, invoc_rate_table[node_from][func]))

                # Note: this is a simple code that tries to solve the problem of more
                # requests that not exactly sum to invocation rate. 
                # Due to the rounding of forwarded requests multiplied for weights 
                # they do not exactly sum to invocation rate.
                #
                # if invoc_rate_table[node_from][func] - s == 1:
                #     weights = [int(round(w)) for w in weights_table[node_from][func].values()]
                #     key = np.random.choice(nodes, 1, p=weights)
                #     table[node_from][func][key] += 1
                #     s = sum(fwd_requests.values())
                #     print("     > Sum for func {} from {} is NOT equal to {} = invoc rate of {} : CHECK FIXED".format(
                #         func, node_from, s, invoc_rate_table[node_from][func]))

def create_tables(fwd_requests, invoc_rate, max_rate, minute, strategy_type):
    path = "test/reports/" + strategy_type + "/minute_" + str(minute) + "/"
    nodes_set = sorted(set(fwd_requests.keys()))
    for func in function_names:
        df_x_func = pd.DataFrame([], index=nodes_set)
        for node_from in fwd_requests:
            df_x_func[node_from] = [fwd_requests[node_from][func][k]
                                    for k in sorted(fwd_requests[node_from][func].keys())]
        df_x_func = df_x_func.T
        df_x_func.to_csv(path + func + ".csv", sep='\t', encoding='utf-8')

        print("     > FWD_TABLE FOR FUNC {}".format(func))
        print(df_x_func)
        
    df_invoc = pd.DataFrame([], index=function_names, columns=nodes_set)
    for node in invoc_rate:
        df_invoc[node] = [invoc_rate[node][f] for f in function_names]
    df_invoc = df_invoc.T
    print("     > INVOC_RATE_TABLE")
    print(df_invoc)

    df_invoc.to_csv(path + "invoc_rates.csv", sep='\t', encoding='utf-8')

    df_max_rates = pd.DataFrame([], index=function_names, columns=nodes_set)
    for node in max_rate:
        df_max_rates[node] = [max_rate[node][f] for f in function_names]

    df_max_rates = df_max_rates.T
    df_max_rates.to_csv(path + "max_rates.csv", sep='\t', encoding='utf-8')
    print("     > MAX_RATE_TABLE")
    print(df_max_rates)

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
    execution_times_bs = {}
    execution_times_rs = {}
    execution_times_es = {}

    # Initialization of execution time map
    for id in range(0, nodes_number):
        execution_times_bs[id] = []
        execution_times_rs[id] = []
        execution_times_es[id] = []

    for minute in range(0, 7): # 6 minutes
        final_config = {}
        
        # Dictionaries used for analysis
        simulation_weights_table = {}
        simulation_invoc_rate_table = {}        
        simulation_max_rate_table = {}
        simulation_weights_table_bs = {}
        simulation_weights_table_rs = {}
        simulation_weights_table_es = {}

        for i, f in zip(range(0, nodes_number), loaded_json):
            key = "node_" + str(i)
            f['output'][minute]['node'] = f['input']['node']
            final_config[key] = f['output'][minute]

            simulation_invoc_rate_table[key] = {}
            simulation_max_rate_table[key] = {}
            for func in final_config[key]["functions"]:
                # Fill tables
                simulation_invoc_rate_table[key][func["name"]] = func["invoc_rate"]
                simulation_max_rate_table[key][func["name"]] = func["max_rate"]
            
        #print(final_config)
        
        # Fill max_rate and invoc_rate table
        for node, weights_x_func in simulation_invoc_rate_table.items():
            for f in function_names:
                if f not in list(weights_x_func.keys()):
                    simulation_invoc_rate_table[node][f] = 0
        
        # Fill max_rate and invoc_rate table
        for node, weights_x_func in simulation_max_rate_table.items():
            for f in function_names:
                if f not in list(weights_x_func.keys()):
                    simulation_max_rate_table[node][f] = 0

        # Write configuration on file
        with open('test/final_config/config{}.json'.format(minute), 'w', encoding='utf-8') as f:
            json.dump(final_config, f, ensure_ascii=False, indent=4)

        # 4) Call agent loop for each config that has been previously built
        # With last update this code is executed for each type of behaviour
        # (base, random and empirical) and for each agent in the network
        for id in range(0, nodes_number):
            #====================== BASE STRATEGY ======================
            # Base strategy agent
            agent = Agent(
                id,
                get_logger(
                    "agent" + str(id) + "_minute_" + str(minute) + "_bs",
                    "test/logs/agent" + str(id) + "_minute_" + str(minute) + "_bs.log"
                ),
                BaseStrategy("", False, final_config)
            )
            agent.disable_logging()
            weights, execution_time = run_agent(agent)
            execution_times_bs[id].append(execution_time)           
            simulation_weights_table_bs["node_"+str(id)] = weights
            #===========================================================

            #====================== RANDOM STRATEGY ====================
            # Random strategy agent
            agent = Agent(
                id,
                get_logger(
                    "agent" + str(id) + "_minute_" + str(minute) + "_rs",
                    "test/logs/agent" + str(id) + "_minute_" + str(minute) + "_rs.log"
                ),
                RandomStrategy("", False, final_config)
            )
            agent.disable_logging()
            weights, execution_time = run_agent(agent)
            execution_times_rs[id].append(execution_time)
            simulation_weights_table_rs["node_"+str(id)] = weights
            #=============================================================

            #====================== EMPIRICAL STRATEGY ===================
            # Empirical strategy agent          
            agent = Agent(
                id,
                get_logger(
                    "agent" + str(id) + "_minute_" + str(minute) + "_es",
                    "test/logs/agent" +
                    str(id) + "_minute_" + str(minute) + "_es.log"
                ),
                EmpiricalStrategy("", False, final_config)
            )
            agent.disable_logging()
            weights, execution_time = run_agent(agent)
            execution_times_es[id].append(execution_time)
            simulation_weights_table_es["node_"+str(id)] = weights
            #============================================================
        
        # Create a table for forwarded requests count for three algorithms
        fwd_requests_bs = xfunc_request_table(
            simulation_max_rate_table, 
            simulation_invoc_rate_table,
            simulation_weights_table_bs
        )

        fwd_requests_rs = xfunc_request_table(
            simulation_max_rate_table, 
            simulation_invoc_rate_table,
            simulation_weights_table_rs
        )

        fwd_requests_es = xfunc_request_table(
            simulation_max_rate_table, 
            simulation_invoc_rate_table,
            simulation_weights_table_es
        ) 

        print("> START MINUTE {}".format(minute))

        # Create and export tables for three algorithms
        print(" > BASE STRATEGY")
        create_tables(fwd_requests_bs, simulation_invoc_rate_table, simulation_max_rate_table, minute, "base_strategy")
        print("------------------------------------------------")
   
        print(" > RANDOM STRATEGY")
        create_tables(fwd_requests_rs, simulation_invoc_rate_table, simulation_max_rate_table, minute, "random_strategy")
        print("------------------------------------------------")

        print(" > EMPIRICAL STRATEGY")
        create_tables(fwd_requests_es, simulation_invoc_rate_table, simulation_max_rate_table, minute, "empirical_strategy")
        print("------------------------------------------------")

        print("> END MINUTE {}".format(minute))

    bs_mean_time = np.mean([np.mean(time_x_agent) for _, time_x_agent in execution_times_bs.items()])
    rs_mean_time = np.mean([np.mean(time_x_agent) for _, time_x_agent in execution_times_rs.items()])
    es_mean_time = np.mean([np.mean(time_x_agent) for _, time_x_agent in execution_times_es.items()])

    return bs_mean_time, rs_mean_time, es_mean_time

def run_agent(agent):
    # time.perf_counter() returns elapsed time in seconds
    # It is the best way to measure performance
    # See: https://www.geeksforgeeks.org/time-perf_counter-function-in-python/
    start = time.perf_counter()
    weights = agent.run()
    end = time.perf_counter()
    execution = end - start
    
    return weights, execution

"""
    TODO: Not still working
"""
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
simulation(3, node1_config, node2_config, node3_config)
