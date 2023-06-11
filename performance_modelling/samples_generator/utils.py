import json
import time
import requests
import itertools

### CONSTANTS ###
PROMETHEUS_QUERY_URL = "http://192.168.49.2:30983/api/v1/query"
PROMETHEUS_QUERY_RANGE_URL = "http://192.168.49.2:30983/api/v1/query_range"

FUNCTION_BODIES = {
  'figlet': 'Hello DFaas!',
  'nodeinfo': '',
  'shasum': 'abcde',
  'env': '',
  'cows': '',
  'nmap': '127.0.0.1',
  "curl": 'http://www.google.com',
  'qrcode-go': 'http://www.google.com'
}

# It generates an array of tuple with every combination of function names.
# Each tuple generated will have a number of functions that goes from min_number_of_functions (included) to max_number_of_functions (excluded).
def generate_functions_combinations(function_list, min_number_of_functions, max_number_of_functions):
  function_tuple_configurations = []
  for i in range(min_number_of_functions, max_number_of_functions):
    combinations = itertools.combinations(function_list, i)
    function_tuple_configurations.extend(list(combinations))
  return function_tuple_configurations

# It generate a list containing all the possible instances of req/s rates used in the vegeta attack
def generate_rates_list(max_rate):
  rates = []
  while(max_rate >= 0):
    rates.append(max_rate)
    max_rate -= 10
  rates.sort()
  return rates

# It returns the string command to perform a vegeta attack given a function name and a req/s rate.
def vegeta_attack(function_name, rate, duration = '30s', format = 'json'):
  if(rate != 0):
    body = FUNCTION_BODIES[function_name]
    target = f'\'{{method: "GET", url: "http://192.168.49.2:31112/function/{function_name}", body: "{body}" | @base64, header: {{"Content-Type": ["text/plain"]}}}}\''
    attack = f'vegeta attack -duration={duration} -rate={rate} -format={format} -timeout=30s | vegeta report --type=json > reports/report-{function_name}-{rate}.json'
    return 'jq -ncM ' + target + ' | ' + attack
  return ''

# It checks if a given configuration is dominant comparad to another one.
# To check if a configuration is dominant it is performed a check on the req/s rate of the functions in the configuration.
# The configuration who has the overall number of req/s rate bigger is the dominant config.
def is_candidate_config_dominanat(actual_dominant_config, candidate_dominant_config):    
  if(actual_dominant_config == None):
    return False
  actual_config_rate = 0
  for tuple in actual_dominant_config:
    actual_config_rate = actual_config_rate + tuple[1]
  
  candidate_config_rate = 0
  for tuple in candidate_dominant_config:
    candidate_config_rate = candidate_config_rate + tuple[1]
  
  if(candidate_config_rate > actual_config_rate):
    return True
  return False

# Retrieve the success rate from the report generated after the vegeta attack.
def retrieve_function_success_rate(function_name, rate):
  if(rate != 0):
    with open(f"reports/report-{function_name}-{rate}.json") as json_file:
      parsed_file = json.load(json_file)
      return float(parsed_file['success'])
  return 1

# Retrieve the number of replicas of the functions deployed.
def retrieve_function_replicas(query_params):
  response = requests.get(PROMETHEUS_QUERY_URL, query_params, verify=False)
  data = response.json()["data"]
  replica = {}
  for result in data["result"]:
    try:
      functionName = result["metric"]["function_name"].split(".")[0]
      replica[functionName] = result["value"][1]
    except:
      continue
  return replica

# This function let the system rest until the values of cpu and ram usage 
# went back to idle values with a 20% deviation.
# By default it waits. at least 10s 
def rest(base_cpu_usage_idle, base_ram_usage_idle, duration):
  time.sleep(10)
  sleep_time_count = 10
  
  cpu_usage, ram_usage = retrieve_node_resources_usage(duration, None, None)
  while(cpu_usage > (base_cpu_usage_idle + (base_cpu_usage_idle * 20/100)) and ram_usage > (base_ram_usage_idle + (base_ram_usage_idle * 20/100)) ):
    time.sleep(10)
    sleep_time_count += 10
    cpu_usage, ram_usage = retrieve_node_resources_usage(duration, None, None)
  
  while(True):
    function_replicas = retrieve_function_replicas({
      'query': ('gateway_service_count')
    })
    for replica in function_replicas.values():
      if int(replica) >= 15:
        time.sleep(10)
        sleep_time_count += 10
        break
    break
  
  print(f'Rest time: {sleep_time_count}s')
  print({ 'cpu_node': cpu_usage, 'ram_usage': ram_usage })
  return cpu_usage, ram_usage, sleep_time_count

# It interrogates Prometheus to retrieve the node CPU and RAM usage in a given time span.
def retrieve_node_resources_usage(time_span, start_time, end_time):
  if(start_time and end_time):
  # CPU USAGE NODE 0% - 800% (8 CORE) https://www.robustperception.io/understanding-machine-cpu-usage/
    cpu_usage = execute_query(PROMETHEUS_QUERY_RANGE_URL, {
      'query': ('100 * sum(1 - rate(node_cpu_seconds_total{mode="idle"}[%s]))' %(time_span)),
      'start': start_time,
      'end': end_time,
      'step': '10s'
    }, True)
    
    # RAM USAGE NODE IN BYTES
    ram_usage = execute_query(PROMETHEUS_QUERY_RANGE_URL, {
      'query': ('avg(avg_over_time(node_memory_MemTotal_bytes[%s]) - avg_over_time(node_memory_MemAvailable_bytes[%s]))' %(time_span, time_span)),
      'start': start_time,
      'end': end_time,
      'step': '10s'
    }, True)
    
    # # AVG CPU USAGE NODE 0% - 100% https://gist.github.com/payam-int/edf977c6af603fee0ce1b05da7792fe7
    # cpu_usage_2 = execute_query({
    #   'query': ('100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle"}[%s])))' %(time_span))
    # })
    
    # # RAM USAGE NODE 0% - 100% https://gist.github.com/payam-int/edf977c6af603fee0ce1b05da7792fe7
    # ram_usage_2 = execute_query({
    #   'query': ('100 * avg(1 - ((avg_over_time(node_memory_MemFree_bytes[%s]) + avg_over_time(node_memory_Cached_bytes[%s]) + avg_over_time(node_memory_Buffers_bytes[%s])) / avg_over_time(node_memory_MemTotal_bytes[%s])))' %(time_span, time_span, time_span, time_span))
    # })
    
    return cpu_usage, ram_usage
  else:
  # CPU USAGE NODE 0% - 800% (8 CORE) https://www.robustperception.io/understanding-machine-cpu-usage/
    cpu_usage = execute_query(PROMETHEUS_QUERY_URL, {
      'query': ('100 * sum(1 - rate(node_cpu_seconds_total{mode="idle"}[%s]))' %(time_span))
    })
    
    # RAM USAGE NODE IN BYTES
    ram_usage = execute_query(PROMETHEUS_QUERY_URL, {
      'query': ('avg(avg_over_time(node_memory_MemTotal_bytes[%s]) - avg_over_time(node_memory_MemAvailable_bytes[%s]))' %(time_span, time_span))
    })
    return cpu_usage, ram_usage

# It interrogates Prometheus to retrieve CPU and RAM usage for each functions in a given time span.
def retrieve_functions_resource_usage(function_names, time_span, start_time, end_time):
  
  if(start_time and end_time):
    # RAM USAGE FUNCTIONS IN BYTES
    ram_usage_per_functions = []
    for function_name in function_names:
      ram_usage_per_functions.append(execute_query(PROMETHEUS_QUERY_RANGE_URL, {
        'query': ('avg_over_time(container_memory_usage_bytes{id=~"^/kubepods.*", container_label_io_kubernetes_container_name="%s"}[%s])' %(function_name, time_span)),
        'start': start_time,
        'end': end_time,
        'step': '10s'
      }, True))
      
    # CPU USAGE PER FUNCTION 0% - 800%
    cpu_usage_per_functions = []
    for function_name in function_names:
      cpu_usage_per_functions.append(execute_query(PROMETHEUS_QUERY_RANGE_URL, {
        'query': ('100 * sum(rate(container_cpu_usage_seconds_total{id=~"^/kubepods.*",container_label_io_kubernetes_container_name="%s"}[%s]))' %(function_name, time_span)),
        'start': start_time,
        'end': end_time,
        'step': '10s'
      }, True))
  else:
    ram_usage_per_functions = []
    for function_name in function_names:
      ram_usage_per_functions.append(0)
      
    cpu_usage_per_functions = []
    for function_name in function_names:
      cpu_usage_per_functions.append(0)
      
  return cpu_usage_per_functions, ram_usage_per_functions

# It permorfs a http request to the Prometheus API
def execute_query(url, query_params, range_query = False):
  timeout = 0
  while True:
    response = requests.get(url, query_params, verify=False)

    if(response.json()["data"]["result"] == []):
      time.sleep(1)
      timeout += 1
      if(timeout > 30):
        raise Exception('timeout')
      continue
    if(range_query):
      result = get_avg_value_from_response(response.json()["data"], 0)
    else:
      result = get_value_from_response(response.json()["data"])
    break
  return result

def get_value_from_response(data):
  return float(data["result"][0]["value"][1])

def get_avg_value_from_response(data, index):
  values = data["result"][index]["values"]
  sum = 0
  for value in values:
    sum += float(value[1])
  return sum / len(values)

# It generate the csv header
def generate_csv_header(function_names):
  csv_header = []
  for function_name in function_names:
    csv_header.append(f'function_{function_name}')
    csv_header.append(f'rate_function_{function_name}')
    csv_header.append(f'success_rate_function_{function_name}')
    csv_header.append(f'cpu_usage_function_{function_name}')
    csv_header.append(f'ram_usage_function_{function_name}')
    csv_header.append(f'replica_{function_name}')
    csv_header.append(f'overloaded_function_{function_name}')
  csv_header.append('cpu_usage_idle_node')
  csv_header.append('cpu_usage_node')
  csv_header.append('ram_usage_idle_node')
  csv_header.append('ram_usage_node')
  csv_header.append('rest_seconds')
  csv_header.append('overloaded_node')
  return csv_header 

# It generate the csv header related to the skipped configurations
def generate_skipped_config_csv_header(function_names):
  csv_header = []
  for function_name in function_names:
    csv_header.append(f'function_{function_name}')
    csv_header.append(f'rate_function_{function_name}')
  return csv_header 