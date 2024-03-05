import time
import sys
import subprocess
import csv
import itertools
from datetime import datetime
import multiprocessing

from utils import *

### CONSTANTS ### 
FUNCTION_NAMES = ['figlet', 'shasum', 'nmap', 'env', 'curl', 'cavecal/eat-memory']
MAX_ITERATION_PER_CONFIG = 3
MAX_RATE = 200
OPENFAAS_SERVICE_IP = "http://192.168.49.2:31112"   

def main():
    print('Argument List:', str(sys.argv))
    max_rate = int(sys.argv[1])
    duration = sys.argv[2]
    num_physical_cpus = multiprocessing.cpu_count()
    print(f"Numero di CPU fisiche: {num_physical_cpus}")
    max_cpu_percentage = num_physical_cpus * 100
    cpu_overload_percentage = (max_cpu_percentage * 80) / 100
    function_combinations = generate_functions_combinations(FUNCTION_NAMES, 3, 4)
    
    print(function_combinations)

    function_tuple_configs = []

    # Read the last configuration from file
    configuration = open('configuration.txt', 'r')
    functions = configuration.readline()

    # Change configuration only if is present in the file
    if len(functions) != 0:
        functions = functions[:-1]
        loads = configuration.readline().split(',')
        functions = tuple(functions.split(','))
        for x in range(function_combinations.index(functions), len(function_combinations)):
            function_tuple_configs.append(function_combinations[x])

        loaded_config = []
        functions_list = list(functions)
        for x in range(0, len(functions)):
            if functions_list[x].__contains__("/"):
                functions_list[x] = functions_list[x].split("/")[1]
            y = (functions_list[x], int(loads[x]))
            loaded_config.append(y)
        loaded_config = tuple(loaded_config)
        print('Configuration found.')
        print('Starting program from: ', loaded_config)
    else:
        function_tuple_configs = function_combinations

    rates = generate_rates_list(max_rate)

    # Obtain current date and current time as string
    current_datetime = datetime.now().strftime('%Y-%m-%d_%H-%M-%S')
        
    batch_iterator = 0
    for function_tuple_config in function_tuple_configs:
        # File location where we will be saving our attack results.
        RESULT_FILE_NAME = f'../output/results-{current_datetime}-{batch_iterator}-{duration}.csv'
        SKIPPED_RESULT_FILE_NAME = f'../output/skipped-{current_datetime}-{batch_iterator}-{duration}.csv'

        # Wait until the attacks are successfully terminated.
        time.sleep(30)

        # Use kubectl to get the OpenFaaS basic-auth secret and decode the password from Base64
        password_cmd = 'kubectl get secret -n openfaas basic-auth -o jsonpath="{.data.basic-auth-password}" | base64 --decode'
        password = subprocess.check_output(password_cmd, shell=True, text=True).strip()

        # Construct the faas-cli login command using the obtained password and OpenFaaS service IP
        faas_login_cmd = f'echo -n {password} | faas-cli login --username admin --password-stdin --gateway {OPENFAAS_SERVICE_IP}'
        # Execute the constructed faas-cli login command
        subprocess.call(faas_login_cmd, shell=True)
        
        # Remove unused deployed functions.
        for function in FUNCTION_NAMES:
            if function.__contains__("/"):
                subprocess.call(['faas-cli', 'remove', function.split("/")[1], '--gateway', OPENFAAS_SERVICE_IP], shell=False)
            else:
                subprocess.call(['faas-cli', 'remove', function, '--gateway', OPENFAAS_SERVICE_IP], shell=False)
        
        # Wait until the functions are successfully removed.
        time.sleep(40)

        # Deploy the functions in function_tuple_config.
        subprocess.call(['./deploy_functions.sh'] + [str(MAX_RATE)] + [str(s) for s in function_tuple_config], shell=False)
        print('Functions deployed')

        function_list_config = list(function_tuple_config)
        for i in range(0, len(function_list_config)):
            if function_list_config[i].__contains__("/"):
                function_list_config[i] = function_list_config[i].split("/")[1]

        function_tuple_config = tuple(function_list_config)
        
        print(function_tuple_config)

        # Wait until the functions are successfully deployed.
        time.sleep(30)

        
        # Retrieve metrics in idle state.
        if(batch_iterator == 0):
            base_cpu_usage_node_idle, base_ram_usage_node_idle, base_ram_usage_node_p_idle, base_power_usage_node_idle = retrieve_node_resources_usage(duration, None, None)
        else:
            base_cpu_usage_node_idle, base_ram_usage_node_idle, base_ram_usage_node_p_idle, base_power_usage_node_idle, rest_seconds = rest(base_cpu_usage_node_idle, base_ram_usage_node_idle, base_power_usage_node_idle, duration)
            
        print('\nCPU, RAM and POWER usage in idle state')
        print({ 'cpu_node': base_cpu_usage_node_idle, 'ram_usage': base_ram_usage_node_idle, 'ram_usage_percentage': base_ram_usage_node_p_idle, 'power_usage': base_power_usage_node_idle})

        
        function_with_rate_combinations = []
        for function_name in function_tuple_config:
            temp = []
            print(function_name, rates)
            for element in itertools.product([function_name], rates):
                temp.append(element)
            function_with_rate_combinations.append(temp)
            
        # Creation of output files
        with open(RESULT_FILE_NAME, 'a') as f:  
            writer = csv.DictWriter(f, fieldnames=generate_csv_header(function_tuple_config))
            writer.writeheader()
            f.close()
            
        with open(SKIPPED_RESULT_FILE_NAME, 'a') as f:  
            writer = csv.DictWriter(f, fieldnames=generate_skipped_config_csv_header(function_tuple_config))
            writer.writeheader()
            f.close()
        
        actual_dominant_config = None
        overload_counter = 0
        config_combinations_total = list(itertools.product(*function_with_rate_combinations))

        config_combinations = []
        if (len(functions) != 0 and batch_iterator == 0):
            for x in range(config_combinations_total.index(loaded_config), len(config_combinations_total)):
                config_combinations.append(config_combinations_total[x])
        else:
            config_combinations = config_combinations_total
        batch_iterator = batch_iterator + 1  
        for config in config_combinations:
            current_functions = []
            attack_configs = []
            print('\n----------------------------------------')
            for attack_data in config:
                # Setup vegeta attack
                function_name = attack_data[0]; invocation_rate = attack_data[1]
                current_functions.append(function_name)
                attack = vegeta_attack(function_name, invocation_rate, duration)
                attack_configs.append(attack)
                print(f'Function {function_name} with {invocation_rate} req/s')
                
            # Check if a configuration is dominant
            if(is_candidate_config_dominanat(actual_dominant_config, config)):
                skipped_config = {}
                for attack_data in config:
                    function_name = attack_data[0]; invocation_rate = attack_data[1]
                    skipped_config[f'function_{function_name}'] = function_name
                    skipped_config[f'rate_function_{function_name}'] = invocation_rate
                    
                    with open(SKIPPED_RESULT_FILE_NAME, 'a') as f: 
                        writer = csv.DictWriter(f, fieldnames=generate_skipped_config_csv_header(function_tuple_config))
                        writer.writerow(skipped_config)
                    
                print('-------------Skip attack---------------')
                continue
            actual_dominant_config = None
            overload_counter = 0
            
            
            try:
                j = 0
                for j in range(0, MAX_ITERATION_PER_CONFIG):
                    # Resting
                    cpu_usage_node_idle, ram_usage_node_idle, ram_usage_node_p_idle, power_usage_node_idle, rest_seconds = rest(base_cpu_usage_node_idle, base_ram_usage_node_idle, base_power_usage_node_idle, duration)
                    start_time = datetime.now().timestamp()
                    # Execute vegeta attacks in parallel
                    processes = [subprocess.Popen(attack, shell=True) for attack in attack_configs]
                    [process.wait() for process in processes]
                    end_time = datetime.now().timestamp()
                    print(f'\nAttack number {j + 1} completed')

                    # Retrieve PIDs of the functions
                    functions_pids, function_replicas = get_functions_pids(current_functions)
                    
                    # Retrieve metrics
                    if(end_time - start_time > int(duration[:-1])):
                        cpu_usage_node, ram_usage_node, ram_usage_p_node, power_usage_node= retrieve_node_resources_usage(duration, start_time, end_time)
                        cpu_usage_per_functions, ram_usage_per_functions, power_usage_per_functions = retrieve_functions_resource_usage(function_tuple_config, functions_pids, duration, start_time, end_time)
                        print("METRICS USING START TIME END TIME")
                    else:
                        cpu_usage_node, ram_usage_node, ram_usage_p_node, power_usage_node = retrieve_node_resources_usage(duration, None, None)
                        cpu_usage_per_functions, ram_usage_per_functions, power_usage_per_functions = retrieve_functions_resource_usage(function_tuple_config, functions_pids, duration, None, None)
                        print("METRICS USING DURATION")

                    
                    result = {} 
                    i = 0
                    avg_success_rate = 0
                    are_there_functions_overloaded = False     
                    for attack_data in config:
                        function_name = attack_data[0]; invocation_rate = attack_data[1]
                        success_rate = retrieve_function_success_rate(function_name, invocation_rate)
                        medium_latency = retrieve_function_medium_latency(function_name, invocation_rate)

                        # Check if a function is overloaded
                        is_function_overloaded = 1 if success_rate < 0.9 or int(function_replicas[function_name]) >= 15 else 0
                        result[f'function_{function_name}'] = function_name
                        result[f'rate_function_{function_name}'] = invocation_rate
                        result[f'success_rate_function_{function_name}'] = '%.3f'%success_rate
                        result[f'cpu_usage_function_{function_name}'] = '%.3f'%cpu_usage_per_functions[i]
                        result[f'ram_usage_function_{function_name}'] = '%.3f'%ram_usage_per_functions[i]
                        result[f'power_usage_function_{function_name}'] = '%.3f'%power_usage_per_functions[i]
                        result[f'replica_{function_name}'] = function_replicas[function_name]
                        result[f'overloaded_function_{function_name}'] = is_function_overloaded
                        result[f'medium_latency_function_{function_name}'] = medium_latency
                        if(is_function_overloaded):
                            are_there_functions_overloaded = True
                        avg_success_rate += success_rate
                        i = i + 1
                    
                    avg_success_rate /= len(function_tuple_config)
                    result['cpu_usage_idle_node'] = '%.3f'%cpu_usage_node_idle
                    result['cpu_usage_node'] = '%.3f'%cpu_usage_node
                    result['ram_usage_idle_node'] = '%.3f'%ram_usage_node_idle
                    result['ram_usage_node'] = '%.3f'%ram_usage_node
                    result['ram_usage_idle_node_percentage'] = '%.3f'%ram_usage_node_p_idle
                    result['ram_usage_node_percentage'] = '%.3f'%ram_usage_p_node
                    result['power_usage_idle_node'] = '%.3f'%power_usage_node_idle
                    result['power_usage_node'] = '%.3f'%power_usage_node
                    
                    result['rest_seconds'] = rest_seconds
                    result['overloaded_node'] = 0
                    
                    # Check if the node is olverloaded
                    if(avg_success_rate < 0.95 or cpu_usage_node > cpu_overload_percentage or ram_usage_p_node > 90 or are_there_functions_overloaded):
                        result['overloaded_node'] = 1
                        overload_counter = overload_counter + 1

                    print(result)
                    # Save configuration result
                    with open(RESULT_FILE_NAME, 'a') as f: 
                        writer = csv.DictWriter(f, fieldnames=generate_csv_header(function_tuple_config))
                        writer.writerow(result)

                    if overload_counter > MAX_ITERATION_PER_CONFIG/2:
                        actual_dominant_config = config
                    print('\n----------------------------------------')
            except Exception as e:
                print(e)
                print('An error occured, the attack is skipped!')
                print('Configuration skipped:')
                for attack_data in config:
                    function_name = attack_data[0]; invocation_rate = attack_data[1]
                    print('%s %s' %(function_name, invocation_rate))
                print('\n----------------------------------------')

if __name__ == "__main__":
    main()
