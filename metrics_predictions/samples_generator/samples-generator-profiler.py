# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import time
import sys
import subprocess
import csv
from datetime import datetime
import multiprocessing

from utils import *

### CONSTANTS ###
MAX_ITERATION_PER_CONFIG = 3
MAX_RATE = 200
OPENFAAS_SERVICE_IP = "http://192.168.49.2:31112"
ALL_FUNCTIONS = ["curl", "nmap", "env", "shasum", "cavecal/eat-memory", "figlet"]
FUNCTION_NAMES = ["curl", "nmap", "env", "shasum", "cavecal/eat-memory", "figlet"]


def main():
    scaphandre = True
    print("Argument List:", str(sys.argv))
    max_rate = int(sys.argv[1])
    duration = sys.argv[2]
    if "--no-scaphandre" in sys.argv:
        scaphandre = False
    num_physical_cpus_cmd = [
        "kubectl",
        "--context=midnode-minikube-context",
        "get",
        "node",
        "-o",
        "jsonpath={.items[0].status.capacity.cpu}",
    ]
    num_physical_cpus = int(
        subprocess.check_output(num_physical_cpus_cmd, text=True).strip()
    )
    print(f"Numero di CPU fisiche: {num_physical_cpus}")
    max_cpu_percentage = num_physical_cpus * 100
    cpu_overload_percentage = (max_cpu_percentage * 80) / 100

    rates = generate_rates_list_profiler(max_rate)

    # Obtain current date and current time as string
    current_datetime = datetime.now().strftime("%Y-%m-%d_%H-%M-%S")

    batch_iterator = 0
    for function in FUNCTION_NAMES:
        # Wait until the attacks are successfully terminated.
        time.sleep(30)

        # Use kubectl to get the OpenFaaS basic-auth secret and decode the password from Base64
        password_cmd = 'kubectl --context=midnode-minikube-context get secret -n openfaas basic-auth -o jsonpath="{.data.basic-auth-password}" | base64 --decode'  # CONTEXT NEEDS TO BE BASED ON THE RECEIVING NODE, CHANGE ACCORDINGLY
        password = subprocess.check_output(password_cmd, shell=True, text=True).strip()

        # Construct the faas-cli login command using the obtained password and OpenFaaS service IP
        faas_login_cmd = f"echo -n {password} | faas-cli login --username admin --password-stdin --gateway {OPENFAAS_SERVICE_IP}"
        # Execute the constructed faas-cli login command
        subprocess.call(faas_login_cmd, shell=True)

        # Remove unused deployed functions.
        for func in ALL_FUNCTIONS:
            if func.__contains__("/"):
                subprocess.call(
                    [
                        "faas-cli",
                        "remove",
                        func.split("/")[1],
                        "--gateway",
                        OPENFAAS_SERVICE_IP,
                    ],
                    shell=False,
                )
            else:
                subprocess.call(
                    ["faas-cli", "remove", func, "--gateway", OPENFAAS_SERVICE_IP],
                    shell=False,
                )

        # Wait until the functions are successfully removed.
        time.sleep(40)

        # Deploy the current function
        subprocess.call(
            ["./deploy_functions.sh"] + [str(MAX_RATE)] + [function], shell=False
        )
        print("Functions deployed")

        if "/" in function:
            function = function.split("/")[-1]

        print(function)

        # Wait until the functions are successfully deployed.
        time.sleep(30)

        # Retrieve metrics in idle state.
        if batch_iterator == 0:
            (
                base_cpu_usage_node_idle,
                base_ram_usage_node_idle,
                base_ram_usage_node_p_idle,
                base_power_usage_node_idle,
            ) = retrieve_node_resources_usage(duration, None, None, scaphandre)
        else:
            (
                base_cpu_usage_node_idle,
                base_ram_usage_node_idle,
                base_ram_usage_node_p_idle,
                base_power_usage_node_idle,
                rest_seconds,
            ) = rest(
                base_cpu_usage_node_idle,
                base_ram_usage_node_idle,
                base_power_usage_node_idle,
                duration,
                scaphandre,
            )

        batch_iterator += 1

        print("\nCPU, RAM and POWER usage in idle state")
        print(
            {
                "cpu_node": base_cpu_usage_node_idle,
                "ram_usage": base_ram_usage_node_idle,
                "ram_usage_percentage": base_ram_usage_node_p_idle,
                "power_usage": base_power_usage_node_idle,
            }
        )

        function_with_rate = [(function, rate) for rate in rates]

        print(function_with_rate)

        # File location where we will be saving our attack results.
        RESULT_FILE_NAME = (
            f"../output/generator/results-{current_datetime}-{function}.csv"
        )

        # Creation of output files
        with open(RESULT_FILE_NAME, "a") as f:
            writer = csv.DictWriter(f, fieldnames=generate_csv_header_one_function())
            writer.writeheader()
            f.close()

        for config in function_with_rate:
            current_functions = []
            attack_configs = []
            print("\n----------------------------------------")

            # Setup vegeta attack
            function_name, invocation_rate = config
            current_functions.append(function_name)
            attack = vegeta_attack(function_name, invocation_rate, duration)
            attack_configs.append(attack)
            print(f"Function {function_name} with {invocation_rate} req/s")

            try:
                j = 0
                for j in range(0, MAX_ITERATION_PER_CONFIG):
                    # Resting
                    (
                        cpu_usage_node_idle,
                        ram_usage_node_idle,
                        ram_usage_node_p_idle,
                        power_usage_node_idle,
                        rest_seconds,
                    ) = rest_for_profiler(
                        base_cpu_usage_node_idle,
                        base_ram_usage_node_idle,
                        base_power_usage_node_idle,
                        duration,
                        scaphandre,
                    )
                    start_time = datetime.now().timestamp()

                    # Execute vegeta attack
                    subprocess.run(attack_configs[0], shell=True).check_returncode()

                    end_time = datetime.now().timestamp()
                    print(f"\nAttack number {j + 1} completed")

                    # Retrieve PIDs of the functions
                    functions_pids, function_replicas = get_functions_pids(
                        current_functions
                    )

                    # Retrieve metrics
                    if end_time - start_time > int(duration[:-1]):
                        (
                            cpu_usage_per_functions,
                            ram_usage_per_functions,
                            power_usage_per_functions,
                        ) = retrieve_function_resource_usage_for_profile(
                            function,
                            functions_pids,
                            duration,
                            start_time,
                            end_time,
                            scaphandre,
                        )
                        print("METRICS USING START TIME END TIME")
                    else:
                        (
                            cpu_usage_per_functions,
                            ram_usage_per_functions,
                            power_usage_per_functions,
                        ) = retrieve_function_resource_usage_for_profile(
                            function, functions_pids, duration, None, None, scaphandre
                        )
                        print("METRICS USING DURATION")

                    result = {}

                    function_name = config[0]
                    invocation_rate = config[1]
                    success_rate = retrieve_function_success_rate(
                        function_name, invocation_rate
                    )
                    medium_latency = retrieve_function_medium_latency(
                        function_name, invocation_rate
                    )

                    # Check if a function is overloaded
                    is_function_overloaded = (
                        1
                        if success_rate < 0.9
                        or int(function_replicas[function_name]) >= 15
                        else 0
                    )
                    result["name"] = function_name
                    result["rate"] = invocation_rate
                    result["success_rate"] = "%.3f" % success_rate
                    result["cpu_usage"] = "%.3f" % cpu_usage_per_functions
                    result["ram_usage"] = "%.3f" % ram_usage_per_functions
                    result["power_usage"] = "%.3f" % power_usage_per_functions
                    result["overloaded"] = is_function_overloaded
                    result["medium_latency"] = medium_latency

                    print(result)
                    # Save configuration result
                    with open(RESULT_FILE_NAME, "a") as f:
                        writer = csv.DictWriter(
                            f, fieldnames=generate_csv_header_one_function()
                        )
                        writer.writerow(result)

                    print("\n----------------------------------------")
            except Exception as e:
                print(e)
                print("An error occured, the attack is skipped!")
                print("Configuration skipped:")
                function_name = config[0]
                invocation_rate = config[1]
                print("%s %s" % (function_name, invocation_rate))
                print("\n----------------------------------------")


if __name__ == "__main__":
    main()
