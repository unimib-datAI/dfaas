# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0-or-later license. See LICENSE and
# AUTHORS file for more information.

import argparse
import csv
import itertools
import logging
import subprocess
import time
import traceback
from datetime import datetime
from pathlib import Path

import utils

# Set logger configuration.
logging.basicConfig(
    level=logging.INFO,
    format="[%(asctime)s %(filename)s:%(lineno)d %(levelname)s] %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)

### CONSTANTS ###
# FUNCTION_NAMES = ['figlet', 'shasum', 'nmap', 'env', 'curl', 'cavecal/eat-memory']
FUNCTION_NAMES = [
    "figlet",
    "shasum",
    "nmap",
    "env",
    "curl",
    "qrcode-go",
    "sentimentanalysis",
    "face-detect-pigo",
    "face-blur",
    "coherent-line-drawing",
    "certinfo",
    "markdown",
    "openfaas-youtube-dl",
    "openfaas-text-to-speech",
]
MAX_RATE = 200


def main():
    # Parse command-line arguments
    parser = argparse.ArgumentParser(description="Samples generator script")
    parser.add_argument(
        "max_rate", type=int, help="Maximum rate for function invocation"
    )
    parser.add_argument("duration", type=str, help="Duration of the attack")
    parser.add_argument("context", type=str, help="Kubernetes context name")
    parser.add_argument(
        "--iterations",
        type=int,
        default=3,
        help="Number of iterations to repeat form the same configuration (default: 3)",
    )
    parser.add_argument(
        "--scaphandre",
        action="store_true",
        default=False,
        help="Enable scaphandre (default: False)",
    )

    args = parser.parse_args()

    max_rate = args.max_rate
    duration = args.duration
    context = args.context
    scaphandre = args.scaphandre
    iterations_per_config = args.iterations

    node_ip = utils.get_node_ip(context)

    openfaas_gateway = f"http://{node_ip}:31112"
    # We need to get the OpenFaaS Gateway password to use faas-cli tool. We
    # assume also it does not change during the execution of this script.
    openfaas_password_cmd = f'kubectl --context={context} get secret -n openfaas basic-auth -o jsonpath="{{.data.basic-auth-password}}" | base64 --decode'
    logging.info(f"Running command: {openfaas_password_cmd}")
    openfaas_password = subprocess.check_output(
        openfaas_password_cmd, shell=True, text=True
    ).strip()

    num_physical_cpus_cmd = [
        "kubectl",
        f"--context={context}",
        "get",
        "node",
        "-o",
        "jsonpath={.items[0].status.capacity.cpu}",
    ]
    num_physical_cpus = int(
        subprocess.check_output(num_physical_cpus_cmd, text=True).strip()
    )
    logging.info(f"Detected CPUs on the node: {num_physical_cpus}")
    max_cpu_percentage = num_physical_cpus * 100
    cpu_overload_percentage = (max_cpu_percentage * 80) / 100

    logging.info(f"Profiled functions: {FUNCTION_NAMES}")
    function_combinations = utils.generate_functions_combinations(FUNCTION_NAMES, 1, 2)
    # function_combinations = generate_functions_combinations(FUNCTION_NAMES, 3, 4)
    # function_combinations = generate_functions_combinations(FUNCTION_NAMES, 2, 3)
    logging.info(
        f"Nr. of func. combinations (without rate): {len(function_combinations)}"
    )

    # Where the CSV files will be saved.
    output_dir = (Path("../output") / context).resolve().absolute()
    output_dir.mkdir(exist_ok=True, parents=True)
    logging.info(f"Output directory created: {output_dir.as_posix()!r}")

    # Where vegeta reports will be saved.
    reports_dir = Path("reports").absolute()
    reports_dir.mkdir(exist_ok=True)
    logging.info(f"Reports directory created: {reports_dir.as_posix()!r}")

    utils.index_csv_init(output_dir)

    function_tuple_configs = []

    # Read the last configuration from file
    configuration = open("configuration.txt", "r")
    functions = configuration.readline()

    # Change configuration only if is present in the file
    loaded_config = []
    if len(functions) != 0:
        functions = functions[:-1]
        loads = configuration.readline().split(",")
        functions = tuple(functions.split(","))
        for x in range(
            function_combinations.index(functions), len(function_combinations)
        ):
            function_tuple_configs.append(function_combinations[x])

        loaded_config = []
        functions_list = list(functions)
        for x in range(0, len(functions)):
            if functions_list[x].__contains__("/"):
                functions_list[x] = functions_list[x].split("/")[1]
            y = (functions_list[x], int(loads[x]))
            loaded_config.append(y)
        loaded_config = tuple(loaded_config)
        logging.info(f"Starting program from: {loaded_config}")
    else:
        function_tuple_configs = function_combinations

    rates = utils.generate_rates_list(max_rate)

    # Obtain current date and current time as string
    current_datetime = datetime.now().strftime("%Y-%m-%d_%H-%M-%S")

    batch_iterator = 0
    for function_tuple_config in function_tuple_configs:
        logging.info(f"Selected configuration (without rates): {function_tuple_config}")

        # File location where we will be saving our attack results.
        RESULT_FILE_NAME = f"../output/{context}/results-{current_datetime}-{batch_iterator}-{duration}.csv"
        SKIPPED_RESULT_FILE_NAME = f"../output/{context}/skipped-{current_datetime}-{batch_iterator}-{duration}.csv"

        # TODO: Remove?
        time.sleep(30)

        # We need to login with faas-cli tool to interact with the remote
        # OpenFaaS Gateway instance. We also need to periodically login because
        # the auth token expires.
        openfaas_login_cmd = f"faas-cli login --password {openfaas_password} --gateway {openfaas_gateway} --tls-no-verify --timeout 60s"
        logging.info(f"Running command: {openfaas_login_cmd}")
        subprocess.run(openfaas_login_cmd.split(), check=True)

        # Remove all deployed functions.
        utils.faas_cli_delete_functions(openfaas_gateway)
        logging.info("All functions deleted!")

        # Deploy the functions of this configuration.
        functions = [str(s) for s in function_tuple_config]
        utils.faas_cli_deploy_functions(functions, openfaas_gateway)
        logging.info(f"Functions deployed: {functions}")

        function_list_config = list(function_tuple_config)
        for i in range(0, len(function_list_config)):
            if function_list_config[i].__contains__("/"):
                function_list_config[i] = function_list_config[i].split("/")[1]

        function_tuple_config = tuple(function_list_config)

        logging.info(function_tuple_config)

        # Retrieve metrics in idle state.
        if batch_iterator == 0:
            (
                base_cpu_usage_node_idle,
                base_ram_usage_node_idle,
                base_ram_usage_node_p_idle,
                base_power_usage_node_idle,
            ) = utils.retrieve_node_resources_usage(
                duration, None, None, scaphandre, node_ip
            )
        else:
            (
                base_cpu_usage_node_idle,
                base_ram_usage_node_idle,
                base_ram_usage_node_p_idle,
                base_power_usage_node_idle,
                rest_seconds,
            ) = utils.rest(
                base_cpu_usage_node_idle,
                base_ram_usage_node_idle,
                base_power_usage_node_idle,
                duration,
                scaphandre,
                node_ip,
            )

        logging.info(
            "CPU, RAM and POWER usage in idle state: {base_cpu_usage_node_idle} {base_ram_usage_node_idle} {base_ram_usage_node_p_idle}% {base_power_usage_node_idle}"
        )

        function_with_rate_combinations = []
        logging.info("Function, Combinations")
        for function_name in function_tuple_config:
            temp = []
            logging.info(f"Function {function_name!r} with rates {rates}")
            for element in itertools.product([function_name], rates):
                temp.append(element)
            function_with_rate_combinations.append(temp)

        # Creation of output files
        logging.info(f"Creation of: {RESULT_FILE_NAME!r}")
        with open(RESULT_FILE_NAME, "a") as f:
            writer = csv.DictWriter(
                f, fieldnames=utils.generate_csv_header(function_tuple_config)
            )
            writer.writeheader()

        logging.info(f"Creation of: {SKIPPED_RESULT_FILE_NAME!r}")
        with open(SKIPPED_RESULT_FILE_NAME, "a") as f:
            writer = csv.DictWriter(
                f,
                fieldnames=utils.generate_skipped_config_csv_header(
                    function_tuple_config
                ),
            )
            writer.writeheader()

        actual_dominant_config = None
        overload_counter = 0
        config_combinations_total = list(
            itertools.product(*function_with_rate_combinations)
        )
        config_combinations_suport = list(
            itertools.product(*function_with_rate_combinations)
        )
        previous_config = -1

        for config in config_combinations_suport:
            if config == loaded_config:
                break
            previous_config = config

        if previous_config != -1:
            for config in config_combinations_total:
                if config == previous_config:
                    break

        batch_iterator = batch_iterator + 1
        for config in config_combinations_total:
            logging.info("----------------------------------------")
            logging.info(f"Current executed configuration: {config}")
            logging.info("----------------------------------------\n")
            current_functions = []
            attack_configs = []

            for attack_data in config:
                # Setup vegeta attack
                function_name = attack_data[0]
                invocation_rate = attack_data[1]
                current_functions.append(function_name)
                attack = utils.vegeta_attack(
                    function_name, invocation_rate, node_ip, duration
                )
                attack_configs.append(attack)
                logging.info(f"Function {function_name} with {invocation_rate} req/s")

            # Check if a configuration is dominant
            if utils.is_candidate_config_dominanat(actual_dominant_config, config):
                skipped_config = {}
                for attack_data in config:
                    function_name = attack_data[0]
                    invocation_rate = attack_data[1]
                    skipped_config[f"function_{function_name}"] = function_name
                    skipped_config[f"rate_function_{function_name}"] = invocation_rate

                    with open(SKIPPED_RESULT_FILE_NAME, "a") as f:
                        writer = csv.DictWriter(
                            f,
                            fieldnames=utils.generate_skipped_config_csv_header(
                                function_tuple_config
                            ),
                        )
                        writer.writerow(skipped_config)

                logging.info("-------------Skip attack---------------")
                continue
            actual_dominant_config = None
            overload_counter = 0

            # Check if the configuration already exists in the index.csv.
            if utils.index_csv_check_config(output_dir, config):
                logging.info(
                    "Configuration already exist in index.csv, skipping attack"
                )
                logging.info("-------------Skip attack---------------")
                continue

            try:
                j = 0
                for j in range(0, iterations_per_config):
                    # Resting
                    (
                        cpu_usage_node_idle,
                        ram_usage_node_idle,
                        ram_usage_node_p_idle,
                        power_usage_node_idle,
                        rest_seconds,
                    ) = utils.rest(
                        base_cpu_usage_node_idle,
                        base_ram_usage_node_idle,
                        base_power_usage_node_idle,
                        duration,
                        scaphandre,
                        node_ip,
                    )
                    start_time = datetime.now().timestamp()
                    # Execute vegeta attacks in parallel
                    processes = [
                        subprocess.Popen(attack, shell=True)
                        for attack in attack_configs
                    ]
                    [process.wait() for process in processes]
                    end_time = datetime.now().timestamp()
                    logging.info(f"Attack number {j + 1} completed")

                    # Retrieve PIDs of the functions
                    functions_pids, function_replicas = utils.get_functions_pids(
                        current_functions, node_ip
                    )

                    # Retrieve metrics
                    if end_time - start_time > int(duration[:-1]):
                        (
                            cpu_usage_node,
                            ram_usage_node,
                            ram_usage_p_node,
                            power_usage_node,
                        ) = utils.retrieve_node_resources_usage(
                            duration, start_time, end_time, scaphandre, node_ip
                        )
                        (
                            cpu_usage_per_functions,
                            ram_usage_per_functions,
                            power_usage_per_functions,
                        ) = utils.retrieve_functions_resource_usage(
                            function_tuple_config,
                            functions_pids,
                            duration,
                            start_time,
                            end_time,
                            scaphandre,
                            node_ip,
                        )
                        logging.info("METRICS USING START TIME END TIME")
                    else:
                        (
                            cpu_usage_node,
                            ram_usage_node,
                            ram_usage_p_node,
                            power_usage_node,
                        ) = utils.retrieve_node_resources_usage(
                            duration, None, None, scaphandre, node_ip
                        )
                        (
                            cpu_usage_per_functions,
                            ram_usage_per_functions,
                            power_usage_per_functions,
                        ) = utils.retrieve_functions_resource_usage(
                            function_tuple_config,
                            functions_pids,
                            duration,
                            None,
                            None,
                            scaphandre,
                            node_ip,
                        )
                        logging.info("METRICS USING DURATION")

                    result = {}
                    i = 0
                    avg_success_rate = 0
                    are_there_functions_overloaded = False
                    for attack_data in config:
                        function_name = attack_data[0]
                        invocation_rate = attack_data[1]
                        success_rate = utils.retrieve_function_success_rate(
                            function_name, invocation_rate
                        )
                        medium_latency = utils.retrieve_function_medium_latency(
                            function_name, invocation_rate
                        )

                        # Check if a function is overloaded
                        is_function_overloaded = (
                            1
                            if success_rate < 0.9
                            or int(function_replicas[function_name]) >= 15
                            else 0
                        )
                        result[f"function_{function_name}"] = function_name
                        result[f"rate_function_{function_name}"] = invocation_rate
                        result[f"success_rate_function_{function_name}"] = (
                            "%.3f" % success_rate
                        )
                        result[f"cpu_usage_function_{function_name}"] = (
                            "%.3f" % cpu_usage_per_functions[i]
                        )
                        result[f"ram_usage_function_{function_name}"] = (
                            "%.3f" % ram_usage_per_functions[i]
                        )
                        result[f"power_usage_function_{function_name}"] = (
                            "%.3f" % power_usage_per_functions[i]
                        )
                        result[f"replica_{function_name}"] = function_replicas[
                            function_name
                        ]
                        result[f"overloaded_function_{function_name}"] = (
                            is_function_overloaded
                        )
                        result[f"medium_latency_function_{function_name}"] = (
                            medium_latency
                        )
                        if is_function_overloaded:
                            are_there_functions_overloaded = True
                        avg_success_rate += success_rate
                        i = i + 1

                    avg_success_rate /= len(function_tuple_config)
                    result["cpu_usage_idle_node"] = "%.3f" % cpu_usage_node_idle
                    result["cpu_usage_node"] = "%.3f" % cpu_usage_node
                    result["ram_usage_idle_node"] = "%.3f" % ram_usage_node_idle
                    result["ram_usage_node"] = "%.3f" % ram_usage_node
                    result["ram_usage_idle_node_percentage"] = (
                        "%.3f" % ram_usage_node_p_idle
                    )
                    result["ram_usage_node_percentage"] = "%.3f" % ram_usage_p_node
                    result["power_usage_idle_node"] = "%.3f" % power_usage_node_idle
                    result["power_usage_node"] = "%.3f" % power_usage_node

                    result["rest_seconds"] = rest_seconds
                    result["overloaded_node"] = 0

                    # Check if the node is olverloaded
                    if (
                        avg_success_rate < 0.95
                        or cpu_usage_node > cpu_overload_percentage
                        or ram_usage_p_node > 90
                        or are_there_functions_overloaded
                    ):
                        result["overloaded_node"] = 1
                        overload_counter = overload_counter + 1

                    logging.info(result)
                    # Save configuration result
                    with open(RESULT_FILE_NAME, "a") as f:
                        writer = csv.DictWriter(
                            f,
                            fieldnames=utils.generate_csv_header(function_tuple_config),
                        )
                        writer.writerow(result)

                    if overload_counter > iterations_per_config / 2:
                        actual_dominant_config = config

                    # Save the executed configuration (+ rates) to index.csv
                    utils.index_csv_add_config(output_dir, config, RESULT_FILE_NAME)

                    logging.info("----------------------------------------")
            except Exception as e:
                traceback.print_exc()
                print(e)
                logging.info("An error occured, the attack is skipped!")
                logging.info("Configuration skipped:")
                for attack_data in config:
                    function_name = attack_data[0]
                    invocation_rate = attack_data[1]
                    logging.info("%s %s" % (function_name, invocation_rate))
                logging.info("----------------------------------------")


if __name__ == "__main__":
    main()
