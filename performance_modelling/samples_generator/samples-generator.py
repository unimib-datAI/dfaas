import time
import sys
import subprocess
import csv
import itertools
from datetime import datetime

from utils import *

### CONSTANTS ### 
FUNCTION_NAMES = ['figlet', 'nodeinfo', 'shasum', 'env', 'cows', 'nmap', 'curl']
MAX_ITERATION_PER_CONFIG = 3
MAX_RATE = 200

def main():
  print('Argument List:', str(sys.argv))
  max_rate = int(sys.argv[1])
  duration = sys.argv[2]
  
  function_tuple_configs = generate_functions_combinations(FUNCTION_NAMES, 2, 4)
  
  print(function_tuple_configs)
  rates = generate_rates_list(max_rate)
    
  batch_iterator = 0
  for function_tuple_config in function_tuple_configs:
    # File location where we will be saving our attack results.
    RESULT_FILE_NAME = f'output/results-15-{batch_iterator}.csv'
    SKIPPED_RESULT_FILE_NAME = f'output/skipped-15-{batch_iterator}.csv'
    
    # Wait until the attacks are successfully terminated.
    time.sleep(30)
    
    # Remove unused deployed functions.
    for function in FUNCTION_NAMES:
      subprocess.call(['faas-cli', 'remove', function], shell=False)
    
    # Wait until the functions are successfully removed.
    time.sleep(30)

    # Deploy the functions in function_tuple_config.
    subprocess.call(['./deploy_functions.sh'] + [str(MAX_RATE)] + [str(s) for s in function_tuple_config], shell=False)
    print('Functions deployed')

    # Wait until the functions are successfully deployed.
    time.sleep(30)
    
    # Retrieve metrics in idle state.
    if(batch_iterator == 0):
      base_cpu_usage_node_idle, base_ram_usage_node_idle = retrieve_node_resources_usage(duration, None, None)
    else:
      base_cpu_usage_node_idle, base_ram_usage_node_idle, rest_seconds = rest(base_cpu_usage_node_idle, base_ram_usage_node_idle, duration)
      
    print('\nCPU and RAM usage in idle state')
    print({ 'cpu_node': base_cpu_usage_node_idle, 'ram_usage': base_ram_usage_node_idle})

    batch_iterator = batch_iterator + 1  
    
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
    config_combinations = list(itertools.product(*function_with_rate_combinations))
    
    for config in config_combinations:
      attack_configs = []
      print('\n----------------------------------------')
      for attack_data in config:
        # Setup vegeta attack
        function_name = attack_data[0]; invocation_rate = attack_data[1]
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
      
      mean_avg_success_rate = 0
      mean_cpu_usage_node = 0
      
      now = datetime.utcnow()
      print(f'Start attack at {now}')
      try:
        j = 0
        for j in range(0, MAX_ITERATION_PER_CONFIG):
          # Resting
          cpu_usage_node_idle, ram_usage_node_idle, rest_seconds = rest(base_cpu_usage_node_idle, base_ram_usage_node_idle, duration)
          start_time = datetime.now().timestamp()
          # Execute vegeta attacks in parallel
          processes = [subprocess.Popen(attack, shell=True) for attack in attack_configs]
          [process.wait() for process in processes]
          end_time = datetime.now().timestamp()
        
          print(f'\nAttack number {j + 1} completed')
          
          # Retrieve metrics
          if(end_time - start_time > int(duration[:-1])):
            cpu_usage_node, ram_usage_node = retrieve_node_resources_usage(duration, start_time, end_time)
            cpu_usage_per_functions, ram_usage_per_functions = retrieve_functions_resource_usage(function_tuple_config, duration, start_time, end_time)
          else:
            cpu_usage_node, ram_usage_node = retrieve_node_resources_usage(duration, None, None)
            cpu_usage_per_functions, ram_usage_per_functions = retrieve_functions_resource_usage(function_tuple_config, duration, None, None)
            
          function_replicas = retrieve_function_replicas({
            'query': ('gateway_service_count')
          })
          
          result = {} 
          i = 0
          avg_success_rate = 0
          are_there_functions_overloaded = False     
          for attack_data in config:
            function_name = attack_data[0]; invocation_rate = attack_data[1]
            success_rate = retrieve_function_success_rate(function_name, invocation_rate)

            is_function_overloaded = 1 if success_rate < 0.9 or int(function_replicas[function_name]) >= 15 else 0
            result[f'function_{function_name}'] = function_name
            result[f'rate_function_{function_name}'] = invocation_rate
            result[f'success_rate_function_{function_name}'] = '%.3f'%success_rate
            result[f'cpu_usage_function_{function_name}'] = '%.3f'%cpu_usage_per_functions[i]
            result[f'ram_usage_function_{function_name}'] = '%.3f'%ram_usage_per_functions[i]
            result[f'replica_{function_name}'] = function_replicas[function_name]
            result[f'overloaded_function_{function_name}'] = is_function_overloaded
            if(is_function_overloaded):
              are_there_functions_overloaded = True
            avg_success_rate += success_rate
            i = i + 1
          
          avg_success_rate /= len(function_tuple_config)
          result['cpu_usage_idle_node'] = '%.3f'%cpu_usage_node_idle
          result['cpu_usage_node'] = '%.3f'%cpu_usage_node
          result['ram_usage_idle_node'] = '%.3f'%ram_usage_node_idle
          result['ram_usage_node'] = '%.3f'%ram_usage_node
          
          result['rest_seconds'] = rest_seconds
          result['overloaded_node'] = 0
          
          if(avg_success_rate < 0.95 or cpu_usage_node > 400 or are_there_functions_overloaded):
            result['overloaded_node'] = 1

          print(result)
          # Save configuration result
          with open(RESULT_FILE_NAME, 'a') as f: 
            writer = csv.DictWriter(f, fieldnames=generate_csv_header(function_tuple_config))
            writer.writerow(result)
        
          mean_avg_success_rate += avg_success_rate
          mean_cpu_usage_node += cpu_usage_node
          
          # Calcute if node is overloaded
          if(j == MAX_ITERATION_PER_CONFIG - 1):
            mean_avg_success_rate /= MAX_ITERATION_PER_CONFIG
            mean_cpu_usage_node /= MAX_ITERATION_PER_CONFIG
            if(mean_avg_success_rate < 0.95 or mean_cpu_usage_node > 400 or are_there_functions_overloaded):
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