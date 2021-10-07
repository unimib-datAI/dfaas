import instance_generator
import simulation
import analyzer
import time
import os
import errno
from datetime import datetime
import pandas as pd
import numpy as np
import random
import shutil
from config_manager import ConfigManager

config_manager = ConfigManager()

def create_folder():
    dir_path = config_manager.SIMULATION_CONTROLLER_ARCHIVE_PATH + \
        datetime.now().strftime('%Y-%m-%d_%H-%M-%S')
    mydir = os.path.join(
        os.getcwd(),
        dir_path
    )
    try:
        os.makedirs(mydir)
    except OSError as e:
        if e.errno != errno.EEXIST:
            raise  # This was not a "directory exist" error..
    return dir_path

def copy_dir(src, dest):
    src_files = os.listdir(src)
    for file_name in src_files:
        full_file_name = os.path.join(src, file_name)
        if os.path.isfile(full_file_name):
            shutil.copy(full_file_name, dest)

def main():
    # Final dataframe containing data for each experiment of the simulation
    final_df = pd.DataFrame()

    # Create folder with timestamp as name for storing
    # simulation data
    dir_path = create_folder()

    print("> STEP 1 - Generating instance configuration...")
    instance_generator.main()

    # Move instance generator output to final folder
    copy_dir(config_manager.OUTPUT_INSTANCE_PATH, dir_path)

    # Execute each instance for a predefined number of times (ex. 5)
    for i in range(0, config_manager.NUMBER_OF_SIMULATION_EXECUTION):

        # Generate a random seed for the experiment
        seed = np.random.randint(0, 4096)
        random.seed(seed)
        np.random.seed(seed)

        print("> STEP 2 - Simulation of instance...")
        simulation.main()
        
        #time.sleep(2)

        print("> STEP 3 - Analyze output...")
        analyzer.main()

        # Create a dir for each iteration of the simulation
        path = dir_path + "/iteration_{}".format(i)
        os.makedirs(path)
        copy_dir(config_manager.ANALYZER_OUTPUT_PATH, path)

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

        # print(final_df)
        # time.sleep(5)

    print("> STEP 5 - Export final results table...")
    print(final_df)

    final_df.to_csv(config_manager.SIMULATION_CONTROLLER_OUTPUT_FILE, sep='\t', encoding='utf-8')

    # Move instance generator output to final folder
    copy_dir(config_manager.SIMULATION_CONTROLLER_OUTPUT_PATH, dir_path)

    with open(dir_path + "/" + config_manager.SIMULATION_CONTROLLER_ARCHIVE_COMPARISON_FILE_NAME, "w") as f:
        # Print mean result of comparison index after 5 executions
        for s in config_manager.STRATEGIES:
            print("---------------------------------------------")
            f.write("---------------------------------------------\n")
            print("> Strategy {}".format(s))
            f.write("> Strategy {}\n".format(s))
            res = final_df.query("strategy == @s")

            for index in config_manager.INDEX_TO_COMPARE:
                val = res[index].mean()      
                print("     > {}: {}".format(index, val))
                f.write("     > {}: {}\n".format(index, val))

# Call main program.
if __name__ == "__main__":
    main()
