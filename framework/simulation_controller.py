import instance_generator
import simulation
import analyzer
import pandas as pd
import numpy as np
import random
from utils.utils import *
from cli.cli import get_simulation_controller_args
from configuration.config_manager import ConfigManager 
import subprocess

config_manager = ConfigManager()

# Create the specific path if not present in the machine    
def create_path_if_not_exists(path):
    if not os.path.exists(path):
        os.makedirs(path)
    return path

def main():
    # Get cli args
    kargs = get_simulation_controller_args()
    print(kargs)
    
    max_percentage_values = kargs["overloaded"]
    if max_percentage_values == None:
        max_percentage_values = [30, 60, 90]
    n_of_experiments = kargs["expnum"]
    if n_of_experiments == -1:
        n_of_experiments = 1
    
    model_type = kargs["modeltype"]
    # Final dataframe containing data for each experiment of the simulation
    final_df = pd.DataFrame()

    # Create folder with timestamp as name for storing
    # simulation data
    dir_path = create_timestamp_folder(config_manager.SIMULATION_CONTROLLER_ARCHIVE_PATH)
    for i in range(1, n_of_experiments + 1):
        exp_path = dir_path.joinpath("exp_{}".format(i))

        seed = kargs["seed"]
        if seed == -1:
            seed = np.random.randint(1000000)
        print("Experiment" + str(i))
        for percentage in max_percentage_values:
            print("Percentage: " + str(percentage))
            # Create a dir for each iteration of the simulation
            path = exp_path.joinpath("simulation_with_load_{}".format(percentage))
            os.makedirs(path)
            
            # 1) Generate instance configuration using the passed parameters
            print("> STEP 1 - Generating instance configuration...")
            subprocess.run(["python", "./instance_generator.py", "-n", str(kargs["nodesnum"]), "-p", str(kargs["edgeprob"]), "-s", str(seed), "-o", str(percentage)])

            # Move instance generator output to final folder
            copy_dir(config_manager.OUTPUT_INSTANCE_PATH, path)

            # Before simulations starts, remove all agent logs file from base foulder
            remove_dir_content(config_manager.SIMULATION_AGENT_LOGGING_BASE_PATH)


            # 2) Single simulation based on configuration file generated before
            print("> STEP 2 - Simulation of instance...")
            subprocess.run(["python", "./simulation.py", "-m", model_type])


            # 3) Analyze simulation output
            print("> STEP 3 - Analyze output...")
            subprocess.run(["python", "./analyzer.py", "-m", model_type])

            
            # Move analyzer output files to final folder (separated for each iteration)
            copy_dir(config_manager.ANALYZER_OUTPUT_PATH, path)
            
            # Copy to this foulder also simulation results (weights for each agent)
            # Also clean src dir of all content (avoiding file overwriting)
            copy_dir(config_manager.SIMULATION_AGENT_LOGGING_BASE_PATH, path)
            remove_dir_content(config_manager.SIMULATION_AGENT_LOGGING_BASE_PATH)

            #time.sleep(2)

            # 4) Load analyzer output file that contains index for comparison
            print("> STEP 4 - Load Index df...")
            df = pd.read_csv(config_manager.INDEX_COMPARISON_FILE,
                            delimiter=',', header=0, index_col=0)
            df.reset_index(drop=False, inplace=True, names = "Strategy")
            #df.index.name = "strategy"
            percentage_values = [percentage] * len(list(df.index.values))
            df.insert(0, 'Max overloaded percentage', percentage_values)
            #df.set_index(["Max overloaded percentage", df.index], inplace=True)
            

            seed_values = [seed] * len(list(df.index.values))
            df.insert(0, 'Seed', seed_values)
            #print(df)

            if final_df.empty:
                final_df = df
            else:
                final_df = pd.concat([final_df, df])

            #print(final_df)
            # time.sleep(5)

    # 5) Export final results comparison table
    print("> STEP 5 - Export final results table...")
    #print(final_df)

    create_path_if_not_exists(config_manager.SIMULATION_CONTROLLER_OUTPUT_PATH)
    final_df.to_csv(config_manager.SIMULATION_CONTROLLER_OUTPUT_FILE, sep=',', encoding='utf-8', index=False)
    #print(final_df)
    
    #df.reset_index(drop=False, inplace=True)
    # Columns to exclude
    columns_to_exclude = ['Seed']

    # Drop the columns to exclude
    df_filtered = final_df.drop(columns=columns_to_exclude)
    #print(df_filtered)
    
    # Group by 'strategy' and 'percentage', then calculate the mean of other columns
    mean_df = df_filtered.groupby(['Max overloaded percentage', 'Strategy']).mean()  
    mean_df.reset_index(drop=False, inplace=True, names = ["Max overloaded percentage", "Strategy"])
    mean_df.insert(0, 'N of experiments', pd.Series([n_of_experiments], index=[0]))

    mean_df.to_csv(config_manager.SIMULATION_CONTROLLER_OUTPUT_MEAN_FILE, sep=',', encoding='utf-8', index=False)


    # Move instance generator output to final folder
    copy_dir(config_manager.SIMULATION_CONTROLLER_OUTPUT_PATH, dir_path)

    # Zip archive foulder with all previous gathered data
    zip_foulder(dir_path, dir_path)

    # Remove dir previously zipped
    remove_dir_with_content(dir_path)

# Call main program.
if __name__ == "__main__":
    main()
