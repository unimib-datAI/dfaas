import json
import numpy as np
from configuration import Configuration

# Active node for the experiment
node = "node_3"

# Map of nodes with specs
nodes = {
    "node_1": { # More simple node
        "ram": 2,
        "cpu": 1
    },
    "node_2": { # Medium power node
        "ram": 4,
        "cpu": 2
    },
    "node_3": { # More powerful node
        "ram": 6, # Not 8 Gb of ram for VB limitations
        "cpu": 4
    }
}

# Express max_rates in terms of r/s for each node
max_rates_for_replica = {
    "node_1": {
        "funca": 15,
        "funcb": 3,
        "funcc": 1 # Maybe 2
    },
    "node_2": {
        "funca": 20,
        "funcb": 5,
        "funcc": 3
    },
    "node_3": {
        "funca": 30,
        "funcb": 10,
        "funcc": 5
    }
}

# Discretizarion of loads percentages
loads_percentages = {
    "low": (0, 34),
    "medium": (33, 67),
    "high": (66, 101),
    "extra-high": (100, 134) # Used for overloaded function state
}

# Replicas limits for differents node types
max_replicas_for_node = {
    "node_1": {
        "funca": (0, 6), # High number of replicas due to semplicity of funca
        "funcb": (0, 3), # Medium number of replicas 
        "funcc": (0, 2)  # More computational expensive -- only one replica
    },
    "node_2": {
        "funca": (0, 6),
        "funcb": (0, 3),
        "funcc": (0, 2)
    },
    "node_3": {
        "funca": (0, 6),
        "funcb": (0, 3),
        "funcc": (0, 2)
    }
}

exp_duration = 5 # Express in term of minutes -- NOT USED

def generate_configuration():
    # Generation of the scenario
    print("Generating a configuration...")
    print("Node used for experiment: " + node)
    print("Node spec used for experiment: ")
    print(nodes[node])

    config = Configuration()
    config.set_node(node)

    replicas = {}
    for k, v in max_replicas_for_node[node].items():
        r = np.random.randint(v[0], v[1])
        print("Replicas for function {} are {}".format(k, r))
        replicas[k] = r

    config.set_funca_replicas(replicas["funca"])    \
        .set_funcb_replicas(replicas["funcb"])      \
        .set_funcc_replicas(replicas["funcc"])

    # The workload and the maxrates have to be calculated starting by the number of replicas
    # Note: suppose that the load is equally distributied on the different replicas
    # For this reason the load is generated for only one replica and then multiplied for 
    # the number of active replicas for this function
    final_loads = {}
    for k, v in max_rates_for_replica[node].items():
        load = np.random.choice(list(loads_percentages.keys()))
        print("{} load for function {}".format(load, k))
        percentage = np.random.randint(loads_percentages[load][0], loads_percentages[load][1])
        effective_load = percentage * v / 100                   # Get percentage of load for replica
        print("Effective load: " + str(effective_load))
        effective_load = round(effective_load * replicas[k])    # Mult. for replica number for this function
        print("Load adjusted for replicas: " + str(effective_load) + " r/s -- Maxrate for replicas: " + str(v*replicas[k]) + "r/s")
        final_loads[k] = effective_load
        print("Load for function {} is {} r/s".format(k, effective_load))

    config.set_funca_wl(final_loads["funca"])    \
        .set_funcb_wl(final_loads["funcb"])      \
        .set_funcc_wl(final_loads["funcc"])
        
    return config

# Utility function
# def is_new_config(config, configurations):
#     for c in configurations:
#         if (c == config):
#             return False
#     return True

def dump_json(config, i):
    json_struct = {
        "input" : { },
        "output": [{ }]
    }

    json_struct["input"] = config.__dict__
    
    json_str = json.dumps(json_struct)
    json_file = open("experiments/exp{}.json".format(i), "w")
    json_file.write(json_str)
    json_file.close()
    
    # Print json indented by a string
    load = json.loads(json_str)
    print(json.dumps(load, indent=4, sort_keys=True))

configurations = []

for i in range(0, 11):
    config = generate_configuration()

    #if (is_new_config(config, configurations)):
    if (config not in configurations):
        print("=================== CONFIGURATION {} ===================".format(i))
        configurations.append(config)
        dump_json(config, i)
        print("========================================================")
    else:
        print("Config already generated --> skip")

