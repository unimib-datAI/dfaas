# SPDX-License-Identifier: AGPL-3.0-or-later
#
# Copyright 2025 The DFaaS Authors. All rights reserved.
#
# This file is licensed under the AGPL-v3.0-or-later license. See LICENSE and
# AUTHORS file for more information.
import argparse
import json
import subprocess
import sys


def get_matching_containers(function_names):
    """
    Return a dictionary that maps the given function names with all containers
    names that contains the given names ("env", "curl"...) and whose command
    contains "fwatchdog" (the OpenFaaS function watchdog).
    """
    cmd = ["docker", "container", "ls", "--format", "{{.Names}},{{.Command}}"]
    result = subprocess.run(cmd, check=True, capture_output=True, text=True)

    # Output is a CSV file similar to this:
    # docker@minikube:~$ docker container ls --format "{{.Names}},{{.Command}}"
    # k8s_env_env-5bfdc674df-t29lh_openfaas-fn_92fe296d-a2f2-42ed-b048-997e65c7cf85_0,"fwatchdog"
    # k8s_POD_env-5bfdc674df-t29lh_openfaas-fn_92fe296d-a2f2-42ed-b048-997e65c7cf85_0,"/pause"
    # k8s_gateway_gateway-76994bb65c-x8wln_openfaas_5cb33630-a029-450d-8773-123841d37d25_5,"./gateway"
    lines = result.stdout.strip().split("\n")
    matches = {fn: [] for fn in function_names}
    for line in lines:
        # Split the row (we know there is at max one comma).
        name, command = line.split(",", 1)

        # Search a match with given function names.
        for fn in function_names:
            if fn in name and "fwatchdog" in command:
                matches[fn].append(name)
                break

    return matches


def get_container_pid(container_name):
    """
    Return the PID of the main process for the given Docker container name.
    """
    cmd = ["docker", "inspect", "-f", "{{.State.Pid}}", container_name]
    result = subprocess.run(cmd, check=True, capture_output=True, text=True)
    return int(result.stdout.strip())


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Outputs as JSON a dict with the given function names and as value"
            "the list of PIDs of associated Docker containers."
        ),
        epilog="Note: this script is automatically executed by samples-generator.py.",
    )
    parser.add_argument(
        "function_names",
        nargs="+",
        help="List of function names (substrings to match in container names).",
    )
    args = parser.parse_args()

    container_by_fn = get_matching_containers(args.function_names)
    output = {}
    for function_name, containers in container_by_fn.items():
        # Accumulate PIDs to an array and later set to the output dict.
        pids = []
        for container_name in containers:
            pids.append(get_container_pid(container_name))

        # At least one PID must exist!
        if pids:
            output[function_name] = pids
        else:
            print(
                f"Error: no running container PIDs found for function {function_name!r}",
                file=sys.stderr,
            )
            sys.exit(2)

    print(json.dumps(output))


if __name__ == "__main__":
    main()
