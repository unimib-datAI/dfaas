# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
#
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import ast
import csv
import itertools
import json
import logging
import subprocess
import sys
import time
from pathlib import Path
from urllib.parse import urlparse

import pandas as pd
import requests
import yaml

textBlob = """Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do
eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim
veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo
consequat. Duis aute irure dolor in reprehenderit in voluptate velit esse cillum
dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident,
sunt in culpa qui officia deserunt mollit anim id est laborum.

Curabitur pretium tincidunt lacus. Nulla gravida orci a odio. Nullam varius,
turpis et commodo pharetra, est eros bibendum elit, nec luctus magna felis
sollicitudin mauris. Integer in mauris eu nibh euismod gravida. Duis ac tellus
et risus vulputate vehicula. Donec lobortis risus a elit. Etiam tempor."""


FUNCTION_BODIES = {
    # See: https://github.com/openfaas/store-functions
    "figlet": "Hello DFaas!",
    "nodeinfo": "",
    "shasum": "abcde",
    "env": "",
    "cows": "",
    "nmap": "127.0.0.1",
    "curl": "http://www.google.com",
    "qrcode-go": "http://www.google.com",
    "eat-memory": "",
    "sentimentanalysis": textBlob,
    "certinfo": "https://google.com",
    "markdown": "# Title\n**Hello** world!",
    # See: https://github.com/esimov/pigo-openfaas
    "face-detect-pigo": "https://raw.githubusercontent.com/esimov/pigo-openfaas/refs/heads/master/pigo-openfaas/samples/nasa.jpg",
    # See: https://github.com/esimov/pigo-openfaas-faceblur
    "face-blur": "https://raw.githubusercontent.com/esimov/pigo-openfaas/refs/heads/master/pigo-openfaas/samples/nasa.jpg",
    # See: https://github.com/esimov/openfaas-coherent-line-drawing
    "coherent-line-drawing": "https://raw.githubusercontent.com/unimib-datAI/dfaas/refs/heads/main/images/Scenario-crop.png",
    # See: https://github.com/openfaas/store-functions
    # Some original functions does not work, so I updated them.
    "openfaas-youtube-dl": "https://www.youtube.com/watch?v=AjwzRdm8jlY",
    # See: https://github.com/ema-pe/openfaas-text-to-speech
    # This is a forked version of rorpage/openfaas-text-to-speech.
    "openfaas-text-to-speech": "Hello World from a virtual machine!",
}


# It generates an array of tuple with every combination of function names.
# Each tuple generated will have a number of functions that goes from min_number_of_functions (included) to max_number_of_functions (excluded).
def generate_functions_combinations(
    function_list, min_number_of_functions, max_number_of_functions
):
    sorted_fn_list = sorted(function_list)
    function_tuple_configurations = []
    for i in range(min_number_of_functions, max_number_of_functions):
        combinations = itertools.combinations(sorted_fn_list, i)
        function_tuple_configurations.extend(list(combinations))
    return function_tuple_configurations


# It generate a list containing all the possible instances of req/s rates used in the vegeta attack
def generate_rates_list(max_rate):
    rates = []
    while max_rate >= 0:
        rates.append(max_rate)
        max_rate -= 10
    rates.sort()
    return rates


# It generate a list for generator profiler
def generate_rates_list_profiler(max_rate):
    rates = []
    while max_rate >= 20:
        rates.append(max_rate)
        max_rate -= 20
    rates.sort()
    return rates


# It returns the string command to perform a vegeta attack given a function name and a req/s rate.
def vegeta_attack(function_name, rate, node_ip, duration="30s", format="json"):
    if rate != 0:
        body = FUNCTION_BODIES[function_name]
        target = f'\'{{method: "GET", url: "http://{node_ip}:31112/function/{function_name}", body: "{body}" | @base64, header: {{"Content-Type": ["text/plain"]}}}}\''
        attack = f"vegeta attack -duration={duration} -rate={rate} -format={format} -timeout=30s | vegeta report --type=json > reports/report-{function_name}-{rate}.json"
        return "jq -ncM " + target + " | " + attack
    return ""


# It checks if a given configuration is dominant comparad to another one.
# To check if a configuration is dominant it is performed a check on the req/s rate of the functions in the configuration.
# The configuration who has the overall number of req/s rate bigger is the dominant config.
def is_candidate_config_dominanat(actual_dominant_config, candidate_dominant_config):
    if actual_dominant_config is None:
        return False
    actual_config_rate = 0
    for tuple in actual_dominant_config:
        actual_config_rate = actual_config_rate + tuple[1]

    candidate_config_rate = 0
    for tuple in candidate_dominant_config:
        candidate_config_rate = candidate_config_rate + tuple[1]

    if candidate_config_rate > actual_config_rate:
        return True
    return False


# Retrieve the success rate from the report generated after the vegeta attack.
def retrieve_function_success_rate(function_name, rate):
    if rate != 0:
        with open(f"reports/report-{function_name}-{rate}.json") as json_file:
            parsed_file = json.load(json_file)
            return float(parsed_file["success"])
    return 1


# Retrieve the medium latency
def retrieve_function_medium_latency(function_name, rate):
    if rate != 0:
        with open(f"reports/report-{function_name}-{rate}.json") as json_file:
            parsed_file = json.load(json_file)
            return int(parsed_file["latencies"]["mean"])
    return 0


# Retrieve the number of replicas of the functions deployed.
def retrieve_function_replicas():
    temp = subprocess.Popen(
        ["faas-cli", "list", "--tls-no-verify"], stdout=subprocess.PIPE
    )
    data = str(temp.communicate())
    rows = data.split("\\n")
    replicas = {}
    for row in rows[1:-1]:
        row = row.split("\\t")
        replicas[row[0].strip()] = row[2].strip()
    return replicas


# This function let the system rest for Sampler Generator
def rest(
    base_cpu_usage_idle,
    base_ram_usage_idle,
    base_power_usage_node_idle,
    duration,
    scaphandre,
    node_ip,
):
    time.sleep(10)
    sleep_time_count = 10

    cpu_usage, ram_usage, ram_usage_p, power_usage = retrieve_node_resources_usage(
        duration, None, None, scaphandre, node_ip
    )
    while (
        cpu_usage > (base_cpu_usage_idle + (base_cpu_usage_idle * 15 / 100))
        or ram_usage > (base_ram_usage_idle + (base_ram_usage_idle * 15 / 100))
        or power_usage
        > (base_power_usage_node_idle + (base_power_usage_node_idle * 15 / 100))
    ):
        time.sleep(5)
        sleep_time_count += 5
        cpu_usage, ram_usage, ram_usage_p, power_usage = retrieve_node_resources_usage(
            duration, None, None, scaphandre, node_ip
        )
        if sleep_time_count > 180:
            sys.exit(
                1
            )  # Exit the script with a non-zero status to indicate an abnormal termination
    wait = True
    while wait:
        wait = False
        function_replicas = retrieve_function_replicas()
        for replica in function_replicas.values():
            if int(replica) >= 2:
                time.sleep(3)
                sleep_time_count += 3
                wait = True

    logging.info(
        f"Rest time: {sleep_time_count}s -> {cpu_usage} CPU, {ram_usage} ({ram_usage_p}) RAM, {power_usage} power"
    )
    return cpu_usage, ram_usage, ram_usage_p, power_usage, sleep_time_count


# This function let the system rest for Sampler Generator Profiler
def rest_for_profiler(
    base_cpu_usage_idle,
    base_ram_usage_idle,
    base_power_usage_node_idle,
    duration,
    scaphandre,
    node_ip,
):
    time.sleep(30)
    sleep_time_count = 10

    cpu_usage, ram_usage, ram_usage_p, power_usage = retrieve_node_resources_usage(
        duration, None, None, scaphandre, node_ip
    )
    while (
        cpu_usage > (base_cpu_usage_idle + (base_cpu_usage_idle * 15 / 100))
        or ram_usage > (base_ram_usage_idle + (base_ram_usage_idle * 15 / 100))
        or power_usage
        > (base_power_usage_node_idle + (base_power_usage_node_idle * 15 / 100))
    ):
        time.sleep(10)
        sleep_time_count += 5
        cpu_usage, ram_usage, ram_usage_p, power_usage = retrieve_node_resources_usage(
            duration, None, None, scaphandre, node_ip
        )
    wait = True
    while wait:
        wait = False
        function_replicas = retrieve_function_replicas()
        for replica in function_replicas.values():
            if int(replica) >= 2:
                time.sleep(3)
                sleep_time_count += 3
                wait = True

    logging.info(
        f"Rest time: {sleep_time_count}s -> {cpu_usage} CPU, {ram_usage} ({ram_usage_p}) RAM, {power_usage} power"
    )
    return cpu_usage, ram_usage, ram_usage_p, power_usage, sleep_time_count


# It interrogates Prometheus to retrieve the node CPU and RAM usage in a given time span.
def retrieve_node_resources_usage(time_span, start_time, end_time, scaphandre, node_ip):
    prometheus_query_range_url = f"http://{node_ip}:30411/api/v1/query_range"
    prometheus_query_url = f"http://{node_ip}:30411/api/v1/query"

    if start_time and end_time:
        # CPU USAGE NODE 0% - 800% (8 CORE) https://www.robustperception.io/understanding-machine-cpu-usage/
        cpu_usage = execute_query(
            prometheus_query_range_url,
            {
                "query": (
                    '100 * sum(1 - rate(node_cpu_seconds_total{mode="idle"}[%s]))'
                    % (time_span)
                ),
                "start": start_time,
                "end": end_time,
                "step": "10s",
            },
            True,
        )

        # RAM USAGE NODE IN BYTES
        ram_usage = execute_query(
            prometheus_query_range_url,
            {
                "query": (
                    "avg(avg_over_time(node_memory_MemTotal_bytes[%s]) - avg_over_time(node_memory_MemAvailable_bytes[%s]))"
                    % (time_span, time_span)
                ),
                "start": start_time,
                "end": end_time,
                "step": "10s",
            },
            True,
        )

        # RAM USAGE NODE IN BYTES
        ram_usage_p = execute_query(
            prometheus_query_range_url,
            {
                "query": (
                    "100 * avg(1 - ((avg_over_time(node_memory_MemFree_bytes[%s]) + avg_over_time(node_memory_Cached_bytes[%s]) + avg_over_time(node_memory_Buffers_bytes[%s])) / avg_over_time(node_memory_MemTotal_bytes[%s])))"
                    % (time_span, time_span, time_span, time_span)
                ),
                "start": start_time,
                "end": end_time,
                "step": "10s",
            },
            True,
        )
        if scaphandre:
            # POWER USAGE NODE
            power_usage = execute_query(
                prometheus_query_range_url,
                {
                    "query": (
                        "avg_over_time(scaph_host_power_microwatts[%s])" % (time_span)
                    ),
                    "start": start_time,
                    "end": end_time,
                    "step": "10s",
                },
                True,
            )
        else:
            power_usage = float("nan")
        return cpu_usage, ram_usage, ram_usage_p, power_usage
    else:
        # CPU USAGE NODE 0% - 800% (8 CORE) https://www.robustperception.io/understanding-machine-cpu-usage/
        cpu_usage = execute_query(
            prometheus_query_url,
            {
                "query": (
                    '100 * sum(1 - rate(node_cpu_seconds_total{mode="idle"}[%s]))'
                    % (time_span)
                )
            },
        )

        # RAM USAGE NODE IN BYTES
        ram_usage = execute_query(
            prometheus_query_url,
            {
                "query": (
                    "avg(avg_over_time(node_memory_MemTotal_bytes[%s]) - avg_over_time(node_memory_MemAvailable_bytes[%s]))"
                    % (time_span, time_span)
                )
            },
        )

        # RAM USAGE NODE 0% - 100% https://gist.github.com/payam-int/edf977c6af603fee0ce1b05da7792fe7
        ram_usage_p = execute_query(
            prometheus_query_url,
            {
                "query": (
                    "100 * avg(1 - ((avg_over_time(node_memory_MemFree_bytes[%s]) + avg_over_time(node_memory_Cached_bytes[%s]) + avg_over_time(node_memory_Buffers_bytes[%s])) / avg_over_time(node_memory_MemTotal_bytes[%s])))"
                    % (time_span, time_span, time_span, time_span)
                )
            },
        )
        if scaphandre:
            # POWER USAGE NODE IN MICROWATTS
            power_usage = execute_query(
                prometheus_query_url, {"query": ("scaph_host_power_microwatts")}
            )
        else:
            power_usage = float("nan")

        return cpu_usage, ram_usage, ram_usage_p, power_usage


# It interrogates Prometheus to retrieve CPU and RAM usage for each functions in a given time span.
def retrieve_functions_resource_usage(
    function_names, functions_pids, time_span, start_time, end_time, scaphandre, node_ip
):
    prometheus_query_range_url = f"http://{node_ip}:30411/api/v1/query_range"

    if start_time and end_time:
        # RAM USAGE FUNCTIONS IN BYTES
        ram_usage_per_functions = []
        for function_name in function_names:
            ram_usage_per_functions.append(
                execute_query(
                    prometheus_query_range_url,
                    {
                        "query": (
                            'avg_over_time(container_memory_usage_bytes{id=~"^/kubepods.*", container_label_io_kubernetes_container_name="%s"}[%s])'
                            % (function_name, time_span)
                        ),
                        "start": start_time,
                        "end": end_time,
                        "step": "10s",
                    },
                    True,
                )
            )

        # CPU USAGE PER FUNCTION 0% - 800%
        cpu_usage_per_functions = []
        for function_name in function_names:
            cpu_usage_per_functions.append(
                execute_query(
                    prometheus_query_range_url,
                    {
                        "query": (
                            '100 * sum(rate(container_cpu_usage_seconds_total{id=~"^/kubepods.*",container_label_io_kubernetes_container_name="%s"}[%s]))'
                            % (function_name, time_span)
                        ),
                        "start": start_time,
                        "end": end_time,
                        "step": "10s",
                    },
                    True,
                )
            )

        # POWER USAGE PER FUNCTION
        power_usage_per_functions = []
        for function_name in function_names:
            if scaphandre:
                pid_list = [str(k) + "|" for k in functions_pids[function_name]]
                pid_str = "".join(pid_list)
                query = f'sum(avg_over_time(scaph_process_power_consumption_microwatts{{pid=~"{pid_str}"}}[{time_span}]))'
                logging.info(query)
                power_usage_per_functions.append(
                    execute_query(
                        prometheus_query_range_url,
                        {
                            "query": (query),
                            "start": start_time,
                            "end": end_time,
                            "step": "10s",
                        },
                        True,
                    )
                )
            else:
                power_usage_per_functions.append(float("nan"))
    else:
        ram_usage_per_functions = []
        for function_name in function_names:
            ram_usage_per_functions.append(0)

        cpu_usage_per_functions = []
        for function_name in function_names:
            cpu_usage_per_functions.append(0)

        power_usage_per_functions = []
        for function_name in function_names:
            if scaphandre:
                power_usage_per_functions.append(0)
            else:
                power_usage_per_functions.append(float("nan"))
    return cpu_usage_per_functions, ram_usage_per_functions, power_usage_per_functions


# It interrogates Prometheus to retrieve CPU and RAM usage for a given function in a given time span.
def retrieve_function_resource_usage_for_profile(
    function_name, function_pids, time_span, start_time, end_time, scaphandre, node_ip
):
    prometheus_query_range_url = f"http://{node_ip}:30411/api/v1/query_range"

    if start_time and end_time:
        # RAM USAGE FUNCTION IN BYTES
        ram_usage = execute_query(
            prometheus_query_range_url,
            {
                "query": (
                    'avg_over_time(container_memory_usage_bytes{id=~"^/kubepods.*", container_label_io_kubernetes_container_name="%s"}[%s])'
                    % (function_name, time_span)
                ),
                "start": start_time,
                "end": end_time,
                "step": "10s",
            },
            True,
        )

        # CPU USAGE FOR FUNCTION (0% - 800%)
        cpu_usage = execute_query(
            prometheus_query_range_url,
            {
                "query": (
                    '100 * sum(rate(container_cpu_usage_seconds_total{id=~"^/kubepods.*",container_label_io_kubernetes_container_name="%s"}[%s]))'
                    % (function_name, time_span)
                ),
                "start": start_time,
                "end": end_time,
                "step": "10s",
            },
            True,
        )

        # POWER USAGE FOR FUNCTION
        if scaphandre:
            pid_list = [str(k) + "|" for k in function_pids[function_name]]
            pid_str = "".join(pid_list)
            query = f'sum(avg_over_time(scaph_process_power_consumption_microwatts{{pid=~"{pid_str}"}}[{time_span}]))'
            logging.info(query)
            power_usage = execute_query(
                prometheus_query_range_url,
                {"query": (query), "start": start_time, "end": end_time, "step": "10s"},
                True,
            )
        else:
            power_usage = float("nan")
    else:
        ram_usage = 0
        cpu_usage = 0
        if scaphandre:
            power_usage = 0
        else:
            power_usage = float("nan")
    return cpu_usage, ram_usage, power_usage


# It permorfs a http request to the Prometheus API
def execute_query(url, query_params, range_query=False):
    timeout = 0
    while True:
        response = requests.get(url, query_params, verify=False)
        if response.json()["data"]["result"] == []:
            time.sleep(1)
            timeout += 1
            if timeout > 30:
                raise Exception("timeout")
            continue
        if range_query:
            result = get_avg_value_from_response(response.json()["data"], 0)
            logging.info(result)
        else:
            result = get_value_from_response(response.json()["data"])
            logging.info(result)
        break
    return result


def safe_execute_query(url, query, default_value=0):
    try:
        return execute_query(url, query)
    except Exception as e:
        logging.error(f"Failed to execute query {query}: {e}")
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
        csv_header.append(f"function_{function_name}")
        csv_header.append(f"rate_function_{function_name}")
        csv_header.append(f"success_rate_function_{function_name}")
        csv_header.append(f"cpu_usage_function_{function_name}")
        csv_header.append(f"ram_usage_function_{function_name}")
        csv_header.append(f"power_usage_function_{function_name}")
        csv_header.append(f"replica_{function_name}")
        csv_header.append(f"overloaded_function_{function_name}")
        csv_header.append(f"medium_latency_function_{function_name}")
    csv_header.append("cpu_usage_idle_node")
    csv_header.append("cpu_usage_node")
    csv_header.append("ram_usage_idle_node")
    csv_header.append("ram_usage_node")
    csv_header.append("ram_usage_idle_node_percentage")
    csv_header.append("ram_usage_node_percentage")
    csv_header.append("power_usage_idle_node")
    csv_header.append("power_usage_node")
    csv_header.append("rest_seconds")
    csv_header.append("overloaded_node")
    return csv_header


# It generate the csv header for profiler
def generate_csv_header_one_function():
    csv_header = []
    csv_header.append("name")
    csv_header.append("rate")
    csv_header.append("success_rate")
    csv_header.append("cpu_usage")
    csv_header.append("ram_usage")
    csv_header.append("power_usage")
    csv_header.append("overloaded")
    csv_header.append("medium_latency")
    return csv_header


# It generate the csv header related to the skipped configurations
def generate_skipped_config_csv_header(function_names):
    csv_header = []
    for function_name in function_names:
        csv_header.append(f"function_{function_name}")
        csv_header.append(f"rate_function_{function_name}")
    return csv_header


def get_functions_pids(function_names, node_ip):
    """
    Return two dictionaries: one that maps each function (by name) to a list of
    PIDs, and one that maps each function to the number of PIDs (replica count).

    The node_ip argument is required and must be the IP address of the remote
    host where the Minikube instance is deployed.
    """
    functions_pids = {}
    functions_replicas = {}

    # We need to connect to the remove VM and then inside the Minikube instance.
    #
    # The custom /etc/find-pid.py script is expected to be added when building
    # the Minikube node!
    cmd = [
        "ssh",
        f"user@{node_ip}",
        "minikube",
        "ssh",
        "python3",
        "/etc/find-pid.py",
    ] + function_names

    logging.info(f"Running command: {' '.join(cmd)}")

    try:
        result = subprocess.run(cmd, capture_output=True, text=True)
    except subprocess.CalledProcessError as e:
        logging.error(
            f"Failed to get PIDs of {function_names} functions. Return code: {e.returncode}"
        )
        if e.stdout:
            logging.error(f"stdout: {e.stdout.strip()}")
        if e.stderr:
            logging.error(f"stderr: {e.stderr.strip()}")
        raise

    # The output is JSON like this:
    # {"env": [4033341], "coherent-line-drawing": [3752040]}
    cmd_output = json.loads(result.stdout.strip())

    for function_name in function_names:
        pids = cmd_output.get(function_name, [])
        functions_pids[function_name] = pids
        functions_replicas[function_name] = len(pids)

    return functions_pids, functions_replicas


def index_csv_init(output_dir):
    """Checks whether index.csv exists in the given output_dir and whether it is
    valid. If it does not exist, initializes a new index.csv file."""
    index_path = Path(output_dir) / "index.csv"
    index_csv_cols = ["functions", "rates", "results_file"]
    # Required since we save lists as columns that use ",".
    index_csv_separator = ";"

    if index_path.is_file():
        # Just try to open the file to check if is valid.
        try:
            pd.read_csv(index_path, sep=";")
            logging.info(f"Index CSV found: {index_path.as_posix()!r}")
        except pd.errors.EmptyDataError as e:
            logging.error(
                f"Index CSV file contains wrong data/header: {index_path.as_posix()!r}: {str(e)}"
            )
            exit(0)
    else:
        # Initialize the file with only header row.
        with index_path.open("w") as index_file:
            writer = csv.writer(index_file, delimiter=index_csv_separator)
            writer.writerow(index_csv_cols)
        logging.info(f"Index CSV file created: {index_path.as_posix()!r}")


def index_csv_add_config(output_dir, config, result_filename):
    """Add the given config to index.csv in the specified output_dir. The
    result_filename string is attached to the config."""
    # Chain .absolute().resolve() needed to get relative paths.
    index_path = Path(output_dir).absolute().resolve() / "index.csv"
    result_filename = Path(result_filename).absolute().resolve()
    result_filename = result_filename.relative_to(index_path.parent)

    index_csv_separator = ";"

    # The config should already be sorted. In any case, we need to split
    # function names and rates, as each is stored in a separate column.
    config = sorted(config, key=lambda x: x[0])
    fn_names, rates = zip(*config)

    # Original values are tuples, but we want to encode lists.
    fn_names, rates = list(fn_names), list(rates)

    # Close the index.csv file immediately to flush buffers and allow other
    # processes to read the file while this program is running.
    with index_path.open("a") as index_file:
        writer = csv.writer(index_file, delimiter=index_csv_separator)
        writer.writerow([fn_names, rates, result_filename])


def index_csv_check_config(output_dir, config):
    """Return True if the given config already exists in the index.csv file
    store in the given output_dir, otherwise return False."""
    # The config should already be sorted. In any case, we need to split
    # function names and rates, as each is stored in a separate column.
    config = sorted(config, key=lambda x: x[0])
    fn_names, rates = zip(*config)

    # Original values are tuples, but we want to encode lists.
    fn_names, rates = list(fn_names), list(rates)

    # Read the index.csv file.
    #
    # Example:
    # functions;rates;results_file
    # ['curl'];[0];results-2025-12-22_16-55-34-0-30s.csv
    # ['curl'];[10];results-2025-12-22_16-55-34-0-30s.csv
    index_path = Path(output_dir).absolute().resolve() / "index.csv"
    df = pd.read_csv(index_path, sep=";")

    # Convert string representations of lists to actual lists
    df["functions"] = df["functions"].apply(ast.literal_eval)
    df["rates"] = df["rates"].apply(ast.literal_eval)

    # Check if there is a row with the same fn_names and rates.
    exists = (
        # Must use apply() because we compare lists, not single elems.
        df["functions"].apply(lambda x: x == fn_names)
        & df["rates"].apply(lambda x: x == rates)
    ).any()

    return exists


def faas_cli_delete_functions(openfaas_gateway):
    """
    Remove all deployed functions on the given OpenFaaS instance.

    Note: it assumes the login is already done.
    """
    cmd_list = f"faas-cli list --quiet --gateway {openfaas_gateway} --tls-no-verify"
    # Get the list of deployed functions.
    try:
        logging.info(f"Running command: {cmd_list}")
        result = subprocess.run(
            cmd_list.split(), capture_output=True, text=True, check=True
        )
    except subprocess.CalledProcessError as e:
        logging.error(
            f"Failed to list OpenFaaS functions (on {openfaas_gateway}). Return code: {e.returncode}"
        )
        if e.stdout:
            logging.error(f"stdout: {e.stdout.strip()}")
        if e.stderr:
            logging.error(f"stderr: {e.stderr.strip()}")
        raise

    functions = result.stdout.strip().splitlines()

    if len(functions) == 0:
        return

    # Remove all deployed functions.
    for function in functions:
        cmd = [
            "faas-cli",
            "remove",
            function,
            "--gateway",
            openfaas_gateway,
            "--tls-no-verify",
        ]
        logging.info(f"Running command: {' '.join(cmd)}")
        try:
            subprocess.run(cmd, check=True, capture_output=True, text=True)
        except subprocess.CalledProcessError as e:
            logging.error(
                f"Failed to remove function {function}. Return code {e.returncode}"
            )
            if e.stdout:
                logging.error(f"stdout: {e.stdout.strip()}")
            if e.stderr:
                logging.error(f"stderr: {e.stderr.strip()}")
            raise

    # Wait that they're really deleted. It may take times, we have a timeout of
    # 2 minutes.
    max = 60
    for i in range(max):
        logging.info(f"Running command [{i + 1}/{max}]: {cmd_list}")
        result = subprocess.run(
            cmd_list.split(), capture_output=True, check=True, text=True
        )
        if not result.stdout.strip():
            break  # No output: all functions have been deleted.
        time.sleep(2)


def faas_cli_deploy_functions(functions, openfaas_gateway):
    """Deploy given OpenFaaS functions to given OpenFaaS Gateway instance (URL)."""
    # Must deploy one by one.
    for function in functions:
        match function:
            case "openfaas-youtube-dl":
                cmd = f"faas-cli deploy --image ghcr.io/ema-pe/openfaas-youtube-dl --name openfaas-youtube-dl --gateway {openfaas_gateway} --tls-no-verify"
            case "openfaas-text-to-speech":
                cmd = f"faas-cli deploy --image ghcr.io/ema-pe/openfaas-text-to-speech --name openfaas-text-to-speech --gateway {openfaas_gateway} --tls-no-verify"
            case _:
                cmd = f"faas-cli store deploy {function} --gateway {openfaas_gateway} --tls-no-verify"

        try:
            logging.info(f"Running command: {cmd}")
            subprocess.run(cmd.split(), check=True)
        except subprocess.CalledProcessError:
            logging.error(f"Failed to deploy function {function!r}")
            raise

    # Then we need to wait the deployments.
    for function in functions:
        ready_cmd = (
            f"faas-cli ready {function} --gateway {openfaas_gateway} --tls-no-verify"
        )
        try:
            logging.info(f"Running command: {ready_cmd}")
            # Blocks until the function has been deployed.
            subprocess.run(ready_cmd.split(), check=True)
        except subprocess.CalledProcessError:
            logging.error(f"Failed readiness check for function {function!r}")
            raise


def get_node_ip(kubectl_context):
    """Return the IP address associated with the given kubectl context name.

    The kubectl context must be present in the ~/.kube/config file, or a
    ValueError exception will be raised.
    """
    # Read and parse YAML config.
    kube_config_path = Path.home() / ".kube" / "config"
    with kube_config_path.open("r") as file:
        config = yaml.safe_load(file)

    # If the given kubectl context exists there must be an entry in the clusters
    # list.
    for cluster in config.get("clusters"):
        if cluster["name"] == kubectl_context:
            url = urlparse(cluster["cluster"]["server"])
            return url.hostname  # Return only the IP address.

    raise ValueError(
        f"Context {kubectl_context!r} not found in {kube_config_path.as_posix()!r}."
    )
