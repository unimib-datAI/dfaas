# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import json
import time
import requests
import itertools
import subprocess
import ast

# Retrieve Prometheus service port
def get_prometheus_service_port():
    service_name = "prometheus"

    # Run the kubectl command to get the information about the service
    command = f"kubectl get svc -n openfaas {service_name} -o json"
    result = subprocess.run(command, shell=True, capture_output=True, text=True)

    if result.returncode != 0:
        print(f"Error while running kubectl: {result.stderr}")
        return None

    # Analyze JSON information
    service_info = json.loads(result.stdout)

    # Extract NodePort
    prometheus_port = next(
        (
            port_info.get("nodePort")
            for port_info in service_info.get("spec", {}).get("ports", [])
            if port_info.get("name") == "web" or port_info.get("name") == "http"
        ),
        None,
    )
    return prometheus_port

# Get the Prometheus NodePort
PROMETHEUS_PORT = 30411
### CONSTANTS ###
PROMETHEUS_SERVICE_IP = "10.99.217.210"

PROMETHEUS_QUERY_URL = f"http://{PROMETHEUS_SERVICE_IP}:{PROMETHEUS_PORT}/api/v1/query"
PROMETHEUS_QUERY_RANGE_URL = f"http://{PROMETHEUS_SERVICE_IP}:{PROMETHEUS_PORT}/api/v1/query_range"

FUNCTION_BODIES = {
    'figlet': 'Hello DFaas!',
    'nodeinfo': '',
    'shasum': 'abcde',
    'env': '',
    'cows': '',
    'nmap': '127.0.0.1',
    "curl": 'http://www.google.com',
    'qrcode-go': 'http://www.google.com',
    'eat-memory': ''
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

# It generate a list for generator profiler
def generate_rates_list_profiler(max_rate):
    rates = []
    while(max_rate >= 20):
        rates.append(max_rate)
        max_rate -= 20
    rates.sort()
    return rates

# It returns the string command to perform a vegeta attack given a function name and a req/s rate.
def vegeta_attack(function_name, rate, duration='30s', format='json'):
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

# Retrieve the medium latency
def retrieve_function_medium_latency(function_name, rate):
    if(rate != 0):
        with open(f"reports/report-{function_name}-{rate}.json") as json_file:
            parsed_file = json.load(json_file)
            return int(parsed_file['latencies']['mean'])
    return 0

# Retrieve the number of replicas of the functions deployed.
def retrieve_function_replicas():
    temp = subprocess.Popen(['faas-cli', 'list'], stdout=subprocess.PIPE)
    data = str(temp.communicate())
    rows = data.split('\\n')
    replicas = {}
    for row in rows[1:-1]:
        row = row.split('\\t')
        replicas[row[0].strip()] = row[2].strip()
    return replicas

# This function let the system rest for Sampler Generator
def rest(base_cpu_usage_idle, base_ram_usage_idle, base_power_usage_node_idle, duration, scaphandre):
    time.sleep(10)
    sleep_time_count = 10

    cpu_usage, ram_usage, ram_usage_p, power_usage = retrieve_node_resources_usage(duration, None, None, scaphandre)
    while(cpu_usage > (base_cpu_usage_idle + (base_cpu_usage_idle * 15/100)) or ram_usage > (base_ram_usage_idle + (base_ram_usage_idle * 15/100)) or power_usage > (base_power_usage_node_idle + (base_power_usage_node_idle * 15/100))):
        time.sleep(5)
        sleep_time_count += 5
        cpu_usage, ram_usage, ram_usage_p, power_usage = retrieve_node_resources_usage(duration, None, None, scaphandre)
    wait = True
    while(wait):
        wait = False
        function_replicas = retrieve_function_replicas()
        for replica in function_replicas.values():
            if int(replica) >= 2:
                time.sleep(3)
                sleep_time_count += 3
                wait = True

    print(f'Rest time: {sleep_time_count}s')
    print({ 'cpu_node': cpu_usage, 'ram_usage': ram_usage, 'ram_usage_percentage': ram_usage_p, 'power_usage': power_usage })
    return cpu_usage, ram_usage, ram_usage_p, power_usage, sleep_time_count

# This function let the system rest for Sampler Generator Profiler
def rest_for_profiler(base_cpu_usage_idle, base_ram_usage_idle, base_power_usage_node_idle, duration, scaphandre):
    time.sleep(30)
    sleep_time_count = 10

    cpu_usage, ram_usage, ram_usage_p, power_usage = retrieve_node_resources_usage(duration, None, None, scaphandre)
    while(cpu_usage > (base_cpu_usage_idle + (base_cpu_usage_idle * 15/100)) or ram_usage > (base_ram_usage_idle + (base_ram_usage_idle * 15/100)) or power_usage > (base_power_usage_node_idle + (base_power_usage_node_idle * 15/100))):
        time.sleep(10)
        sleep_time_count += 5
        cpu_usage, ram_usage, ram_usage_p, power_usage = retrieve_node_resources_usage(duration, None, None, scaphandre)
    wait = True
    while(wait):
        wait = False
        function_replicas = retrieve_function_replicas()
        for replica in function_replicas.values():
            if int(replica) >= 2:
                time.sleep(3)
                sleep_time_count += 3
                wait = True

    print(f'Rest time: {sleep_time_count}s')
    print({ 'cpu_node': cpu_usage, 'ram_usage': ram_usage, 'ram_usage_percentage': ram_usage_p, 'power_usage': power_usage })
    return cpu_usage, ram_usage, ram_usage_p, power_usage, sleep_time_count

# It interrogates Prometheus to retrieve the node CPU and RAM usage in a given time span.
def retrieve_node_resources_usage(time_span, start_time, end_time, scaphandre):
    if(start_time and end_time):
        # CPU USAGE NODE 0% - 800% (8 CORE) https://www.robustperception.io/understanding-machine-cpu-usage/
        cpu_usage = execute_query(PROMETHEUS_QUERY_RANGE_URL, {
            'query': ('100 * sum(1 - rate(node_cpu_seconds_total{mode="idle"}[%s]))' % (time_span)),
            'start': start_time,
            'end': end_time,
            'step': '10s'
        }, True)

        # RAM USAGE NODE IN BYTES
        ram_usage = execute_query(PROMETHEUS_QUERY_RANGE_URL, {
            'query': ('avg(avg_over_time(node_memory_MemTotal_bytes[%s]) - avg_over_time(node_memory_MemAvailable_bytes[%s]))' % (time_span, time_span)),
            'start': start_time,
            'end': end_time,
            'step': '10s'
        }, True)

        # RAM USAGE NODE IN BYTES
        ram_usage_p = execute_query(PROMETHEUS_QUERY_RANGE_URL, {
            'query': ('100 * avg(1 - ((avg_over_time(node_memory_MemFree_bytes[%s]) + avg_over_time(node_memory_Cached_bytes[%s]) + avg_over_time(node_memory_Buffers_bytes[%s])) / avg_over_time(node_memory_MemTotal_bytes[%s])))' % (time_span, time_span, time_span, time_span)),
            'start': start_time,
            'end': end_time,
            'step': '10s'
        }, True)
        if scaphandre == True:
            # POWER USAGE NODE
            power_usage = execute_query(PROMETHEUS_QUERY_RANGE_URL, {
                'query': ('avg_over_time(scaph_host_power_microwatts[%s])' % (time_span)),
                'start': start_time,
                'end': end_time,
                'step': '10s'
            }, True)
        else:
            power_usage = float('nan')
        return cpu_usage, ram_usage, ram_usage_p, power_usage
    else:
        # CPU USAGE NODE 0% - 800% (8 CORE) https://www.robustperception.io/understanding-machine-cpu-usage/
        cpu_usage = execute_query(PROMETHEUS_QUERY_URL, {
            'query': ('100 * sum(1 - rate(node_cpu_seconds_total{mode="idle"}[%s]))' % (time_span))
        })

        # RAM USAGE NODE IN BYTES
        ram_usage = execute_query(PROMETHEUS_QUERY_URL, {
            'query': ('avg(avg_over_time(node_memory_MemTotal_bytes[%s]) - avg_over_time(node_memory_MemAvailable_bytes[%s]))' % (time_span, time_span))
        })

        # RAM USAGE NODE 0% - 100% https://gist.github.com/payam-int/edf977c6af603fee0ce1b05da7792fe7
        ram_usage_p = execute_query(PROMETHEUS_QUERY_URL, {
            'query': ('100 * avg(1 - ((avg_over_time(node_memory_MemFree_bytes[%s]) + avg_over_time(node_memory_Cached_bytes[%s]) + avg_over_time(node_memory_Buffers_bytes[%s])) / avg_over_time(node_memory_MemTotal_bytes[%s])))' % (time_span, time_span, time_span, time_span))
        })
        if scaphandre == True:
            # POWER USAGE NODE IN MICROWATTS
            power_usage = execute_query(PROMETHEUS_QUERY_URL, {
                'query': ('scaph_host_power_microwatts')
            })
        else:
            power_usage = float('nan')

        return cpu_usage, ram_usage, ram_usage_p, power_usage

# It interrogates Prometheus to retrieve CPU and RAM usage for each functions in a given time span.
def retrieve_functions_resource_usage(function_names, functions_pids, time_span, start_time, end_time, scaphandre):

    if(start_time and end_time):
        # RAM USAGE FUNCTIONS IN BYTES
        ram_usage_per_functions = []
        for function_name in function_names:
            ram_usage_per_functions.append(execute_query(PROMETHEUS_QUERY_RANGE_URL, {
                'query': ('avg_over_time(container_memory_usage_bytes{id=~"^/kubepods.*", container_label_io_kubernetes_container_name="%s"}[%s])' % (function_name, time_span)),
                'start': start_time,
                'end': end_time,
                'step': '10s'
            }, True))

        # CPU USAGE PER FUNCTION 0% - 800%
        cpu_usage_per_functions = []
        for function_name in function_names:
            cpu_usage_per_functions.append(execute_query(PROMETHEUS_QUERY_RANGE_URL, {
                'query': ('100 * sum(rate(container_cpu_usage_seconds_total{id=~"^/kubepods.*",container_label_io_kubernetes_container_name="%s"}[%s]))' % (function_name, time_span)),
                'start': start_time,
                'end': end_time,
                'step': '10s'
            }, True))

        # POWER USAGE PER FUNCTION
        power_usage_per_functions = []
        for function_name in function_names:
            if scaphandre == True:
                pid_list = [str(k) + '|' for k in functions_pids[function_name]]
                pid_str = ''.join(pid_list)
                query = f'sum(avg_over_time(scaph_process_power_consumption_microwatts{{pid=~"{pid_str}"}}[{time_span}]))'
                print(query)
                power_usage_per_functions.append(execute_query(PROMETHEUS_QUERY_RANGE_URL, {
                    'query': (query),
                    'start': start_time,
                    'end': end_time,
                    'step': '10s'
                }, True))
            else:
                power_usage_per_functions.append(float('nan'))
    else:
        ram_usage_per_functions = []
        for function_name in function_names:
            ram_usage_per_functions.append(0)

        cpu_usage_per_functions = []
        for function_name in function_names:
            cpu_usage_per_functions.append(0)

        power_usage_per_functions = []
        for function_name in function_names:
            if scaphandre == True:
                power_usage_per_functions.append(0)
            else:
                power_usage_per_functions.append(float('nan'))
    return cpu_usage_per_functions, ram_usage_per_functions, power_usage_per_functions

# It interrogates Prometheus to retrieve CPU and RAM usage for a given function in a given time span.
def retrieve_function_resource_usage_for_profile(function_name, function_pids, time_span, start_time, end_time, scaphandre):
    if start_time and end_time:
        # RAM USAGE FUNCTION IN BYTES
        ram_usage = execute_query(PROMETHEUS_QUERY_RANGE_URL, {
            'query': ('avg_over_time(container_memory_usage_bytes{id=~"^/kubepods.*", container_label_io_kubernetes_container_name="%s"}[%s])' % (function_name, time_span)),
            'start': start_time,
            'end': end_time,
            'step': '10s'
        }, True)

        # CPU USAGE FOR FUNCTION (0% - 800%)
        cpu_usage = execute_query(PROMETHEUS_QUERY_RANGE_URL, {
            'query': ('100 * sum(rate(container_cpu_usage_seconds_total{id=~"^/kubepods.*",container_label_io_kubernetes_container_name="%s"}[%s]))' % (function_name, time_span)),
            'start': start_time,
            'end': end_time,
            'step': '10s'
        }, True)

        # POWER USAGE FOR FUNCTION
        if scaphandre == True:
            pid_list = [str(k) + '|' for k in function_pids[function_name]]
            pid_str = ''.join(pid_list)
            query = f'sum(avg_over_time(scaph_process_power_consumption_microwatts{{pid=~"{pid_str}"}}[{time_span}]))'
            print(query)
            power_usage = execute_query(PROMETHEUS_QUERY_RANGE_URL, {
                'query': (query),
                'start': start_time,
                'end': end_time,
                'step': '10s'
            }, True)
        else:
            power_usage = float('nan')
    else:
        ram_usage = 0
        cpu_usage = 0
        if scaphandre == True:
            power_usage = 0
        else:
            power_usage = float('nan')
    return cpu_usage, ram_usage, power_usage

# It permorfs a http request to the Prometheus API
def execute_query(url, query_params, range_query=False):
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
            print(result)
        else:
            result = get_value_from_response(response.json()["data"])
            print(result)
        break
    return result

def safe_execute_query(url, query, default_value=0):
    try:
        return execute_query(url, query)
    except Exception as e:
        print(f"Failed to execute query {query}: {e}")
        return default_value

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
        csv_header.append(f'power_usage_function_{function_name}')
        csv_header.append(f'replica_{function_name}')
        csv_header.append(f'overloaded_function_{function_name}')
        csv_header.append(f'medium_latency_function_{function_name}')
    csv_header.append('cpu_usage_idle_node')
    csv_header.append('cpu_usage_node')
    csv_header.append('ram_usage_idle_node')
    csv_header.append('ram_usage_node')
    csv_header.append('ram_usage_idle_node_percentage')
    csv_header.append('ram_usage_node_percentage')
    csv_header.append('power_usage_idle_node')
    csv_header.append('power_usage_node')
    csv_header.append('rest_seconds')
    csv_header.append('overloaded_node')
    return csv_header 

# It generate the csv header for profiler
def generate_csv_header_one_function():
    csv_header = []
    csv_header.append('name')
    csv_header.append('rate')
    csv_header.append('success_rate')
    csv_header.append('cpu_usage')
    csv_header.append('ram_usage')
    csv_header.append('power_usage')
    csv_header.append('overloaded')
    csv_header.append('medium_latency')
    return csv_header 

# It generate the csv header related to the skipped configurations
def generate_skipped_config_csv_header(function_names):
    csv_header = []
    for function_name in function_names:
        csv_header.append(f'function_{function_name}')
        csv_header.append(f'rate_function_{function_name}')
    return csv_header 

# Execute the find-pid.py script into minikube to obtain the PIDs of the functions
def get_functions_pids(functions_names):
    #print(len(functions_replicas.keys()), *[str(k) for k in functions_replicas.keys()], *[str(v) for v in functions_replicas.values()])
    temp = subprocess.Popen(['docker', 'exec', '-ti', 'minikube', 'python3', 'etc/find-pid.py', *functions_names], stdout = subprocess.PIPE)
    data = str(temp.communicate())
    data = data.split('\\n')
    if (data[1].__contains__('List of PIDs for the requested functions')):
        output = data[1].split('\\r')
        functions_pids = ast.literal_eval(output[0][43:])
    else:
        print("Log of finding PIDs function:", data)
        raise Exception('Something went wrong in finding PIDs for the functions')
    functions_replicas = {}
    for name in functions_names:
        functions_replicas[name] = len(functions_pids[name])
    print("Replicas of functions:", functions_replicas)
    print("List of PIDs for the requested functions: ", functions_pids)
    return functions_pids, functions_replicas
