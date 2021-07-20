import logging
import time
import pandas as pd
from agent import Agent
from os import listdir
from os.path import isfile, join
import matplotlib.pyplot as plt

#configuration = "2.json"
#agent_num = 6

mypath = "config/experiment/"
experiment_files = sorted([f for f in listdir(mypath) if isfile(join(mypath, f))])
execution_times = []

print(experiment_files)

def get_logger(name, log_file, level=logging.INFO):
    handler = logging.FileHandler(log_file)
    logger = logging.getLogger(name)
    logger.setLevel(level)
    logger.addHandler(handler)

    return logger

# I want to run a single agent (node_0 agent) that do calcolous based on his situation
# Note that agents is not more a thread for timing
for configuration in experiment_files:
    # This run takes into account of agent file logs
    a = Agent(0, join(mypath, configuration), get_logger("agent" + str(0), "node_" + str(0) + ".log"))
    
    # time.perf_counter() returns elapsed time in seconds
    # It is the best way to measure performance
    # See: https://www.geeksforgeeks.org/time-perf_counter-function-in-python/
    start = time.perf_counter()
    a.loop()
    end = time.perf_counter()
    execution = end - start

    execution_times.append(execution)

    print(configuration)
    print(execution)

df = pd.DataFrame()

df["experiment"] = experiment_files
df["time"] = execution_times

print(experiment_files)
print(execution_times)

print(df)

df.plot()

df.to_csv('out.csv')

plt.figure(figsize=(20, 10))
plt.title("Agent execution time in function of the p2p net size or the number of neighbours")
plt.xlabel("Experiment")
plt.ylabel("MAPE loop -- execution time")

plt.plot(df["experiment"], df["time"])
#plt.legend(loc="upper right")
plt.grid()

plt.savefig("prova.png")

# for i in range(0, agent_num):
#     a = Agent(i, configuration, get_logger("agent" + str(i), "node_" + str(i) + ".log"))
#     a.start()

# agent0 = Agent(0, configuration, get_logger("agent0", "node_0.log"))
# agent0.start()

# agent1 = Agent(1, configuration, get_logger("agent1", "node_1.log"))
# agent1.start()

# agent2 = Agent(2, configuration, get_logger("agent2", "node_2.log"))
# agent2.start()

# agent3 = Agent(3, configuration, get_logger("agent3", "node_3.log"))
# agent3.start()

    
