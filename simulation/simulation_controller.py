import instance_generator
import simulation
import analyzer
import time
import pandas as pd
import numpy as np
import random
from config_manager import ConfigManager

config_manager = ConfigManager()

def main():
    # Final dataframe containing data for each experiment of the simulation
    final_df = pd.DataFrame()
    
    print("> STEP 1 - Generating instance configuration...")
    instance_generator.main()

    #time.sleep(2)

    # Execute each instance for a predefined number of times (ex. 5)
    for _ in range(0, config_manager.NUMBER_OF_SIMULATION_EXECUTION):

        # Generate a random seed for the experiment
        seed = np.random.randint(0, 4096)
        random.seed(seed)
        np.random.seed(seed)

        print("> STEP 2 - Simulation of instance...")
        simulation.main()
        
        #time.sleep(2)

        print("> STEP 3 - Analyze output...")
        analyzer.main()

        #time.sleep(2)

        print("> STEP 4 - Load Index df...")
        df = pd.read_csv(config_manager.INDEX_COMPARISON_FILE,
                         delimiter='\t', header=0, index_col=0)

        df.index.name = "strategy"
        df["seed"] = [seed] * len(list(df.index.values))
        df.set_index(["seed", df.index], inplace=True)
        
        print(df)
        
        if final_df.empty:
            final_df = df
        else:
            final_df = pd.concat([final_df, df])

        print(final_df)
        time.sleep(5)

    print("> STEP 5 - Export final results table...")
    print(final_df)

    final_df.to_csv(config_manager.SIMULATION_CONTROLLER_OUTPUT_FILE, sep='\t', encoding='utf-8')

    # Print mean result of comparison index after 5 executions
    for s in config_manager.STRATEGIES:
        print("---------------------------------------------")
        print("> Strategy {}".format(s))
        res = final_df.query("strategy == @s")

        for index in config_manager.INDEX_TO_COMPARE:
            val = res[index].mean()      
            print("     > {}: {}".format(index, val))

# Call main program.
if __name__ == "__main__":
    main()
