#!/usr/bin/env -S uv run --script
# SPDX-License-Identifier: AGPL-3.0-or-later.
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# This is a small CLI utility to configure bidirectional artificial network
# latency between Linux VMs defined in a YAML configuration file.
#
# The script assumes that it can access the VMs via SSH in non-interactive
# mode (without a password, using only a key) and that the user has sudo
# permissions without being prompted for a password.
#
# Run the script with the --help flag for more details.

# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "pyyaml>=6.0.3",
# ]
# ///

import argparse
import os
import shlex
import subprocess
import sys
from pathlib import Path

import yaml

# Global tc handle used as the root qdisc handle.
TC_ROOT_HANDLE = "10:"

# Starting priority for destination-based tc filters.
TC_FILTER_PRIO_BASE = 100


def ssh_command(vm, command):
    """Execute a command on a remote VM using the system ssh executable."""
    host = vm["host"]
    user = vm["user"]

    target = f"{user}@{host}"

    # We need to configure the settings for non-interactive access.
    cmd = ["ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=accept-new"]

    if "port" in vm:
        cmd += ["-p", str(vm["port"])]

    if "key" in vm:
        cmd += ["-i", os.path.expanduser(vm["key"])]

    cmd += [target, "--", command]

    print(f"  SSH {target}: {command}")

    result = subprocess.run(cmd, text=True, capture_output=True, check=False)
    if result.returncode != 0:
        raise RuntimeError(
            f"SSH command failed on {target}\n"
            f"Command: {command}\n"
            f"stdout:\n{result.stdout}\n"
            f"stderr:\n{result.stderr}"
        )

    return result.stdout.strip()


def get_vm(config, name):
    try:
        return config["vms"][name]
    except KeyError:
        raise RuntimeError(f"Unknown VM: {name}")


def detect_interface(vm):
    """Find the interface used to reach the default route."""

    command = (
        "ip -o route get 1.1.1.1 "
        "| awk '{for(i=1;i<=NF;i++) if($i==\"dev\") {print $(i+1); exit}}'"
    )

    interface = ssh_command(vm, command)

    if not interface:
        raise RuntimeError(f"Could not determine network interface for {vm['host']}")

    return interface.strip()


def quote(value):
    """Shell-quote a value."""

    return shlex.quote(str(value))


def build_tc_command(interface, links):
    """
    Build the remote tc command for one VM.

    Traffic is classified by destination IPv4 address.

    Example:

        VM1 -> VM2 = 100ms
        VM1 -> VM3 = 250ms

    becomes:

        destination VM2 -> netem 100ms
        destination VM3 -> netem 250ms
        everything else -> no delay
    """

    bands = len(links) + 1

    commands = [
        # Remove the previous root qdisc.
        (f"tc qdisc del dev {quote(interface)} root 2>/dev/null || true"),
        # Create the root priority qdisc.
        (
            f"tc qdisc add dev {quote(interface)} "
            f"root handle {TC_ROOT_HANDLE} "
            f"prio bands {bands}"
        ),
    ]

    # Create one netem qdisc for every destination.
    for index, link in enumerate(links, start=1):
        delay = link.get("delay", "0ms")
        jitter = link.get("jitter", "0ms")
        loss = link.get("loss", "0%")

        options = [
            "netem",
            "delay",
            quote(delay),
        ]

        if jitter != "0ms":
            options.append(quote(jitter))

        if loss != "0%":
            options.extend(["loss", quote(loss)])

        commands.append(
            f"tc qdisc add dev {quote(interface)} "
            f"parent {TC_ROOT_HANDLE}{index} " + " ".join(options)
        )

    # Classify packets based on destination IP.
    for index, link in enumerate(links, start=1):
        destination = link["destination"]

        commands.append(
            f"tc filter add dev {quote(interface)} "
            "protocol ip "
            f"parent {TC_ROOT_HANDLE} "
            f"prio {TC_FILTER_PRIO_BASE + index} "
            f"flower dst_ip {quote(destination)} "
            f"flowid {TC_ROOT_HANDLE}{index}"
        )

    # Execute all commands through sudo.
    return " && ".join(f"sudo -n {command}" for command in commands)


def build_clear_command(interface):
    """Build the command that removes the root tc qdisc."""

    return f"sudo -n tc qdisc del dev {quote(interface)} root 2>/dev/null || true"


def build_show_command(interface):
    """Build the command that displays tc configuration."""

    return (
        f"tc -s qdisc show dev {quote(interface)}; "
        "echo '--- FILTERS ---'; "
        f"tc filter show dev {quote(interface)}"
    )


def expand_links(config):
    """
    Expand bidirectional links into per-VM outgoing links.

    The VM ``host`` address is used as the destination IP.

    Input:
        vm1 <-> vm2

    Output:
        vm1 -> vm2
        vm2 -> vm1
    """
    result = {name: [] for name in config["vms"]}

    for link in config.get("links", []):
        endpoints = link["endpoints"]
        if len(endpoints) != 2:
            raise RuntimeError(f"Link must contain exactly two endpoints: {link}")

        a, b = endpoints
        vm_a = get_vm(config, a)
        vm_b = get_vm(config, b)

        params = {key: value for key, value in link.items() if key != "endpoints"}

        result[a].append(
            {
                **params,
                "destination": vm_b["host"],
                "peer": b,
            }
        )

        result[b].append(
            {
                **params,
                "destination": vm_a["host"],
                "peer": a,
            }
        )

    return result


def command_apply(config):
    expanded = expand_links(config)

    for name, vm in config["vms"].items():
        print(f"\n=== {name} ({vm['host']}) ===")

        interface = detect_interface(vm)

        print(f"  Interface: {interface}")

        links = expanded[name]

        if not links:
            print("  No configured links.")
            continue

        command = build_tc_command(interface, links)

        print("\n  Applying tc configuration...")

        ssh_command(vm, command)

        print(f"  Applied successfully to {name}")


def command_clear(config):
    for name, vm in config["vms"].items():
        print(f"\n=== {name} ({vm['host']}) ===")

        interface = detect_interface(vm)

        ssh_command(
            vm,
            build_clear_command(interface),
        )

        print(f"  Cleared {interface}")


def command_status(config):
    for name, vm in config["vms"].items():
        print(f"\n{'=' * 60}")
        print(f"{name} ({vm['host']})")

        interface = detect_interface(vm)

        output = ssh_command(
            vm,
            build_show_command(interface),
        )

        print(output)


def command_test(config):
    """Ping every configured link in both directions."""

    for link in config.get("links", []):
        a, b = link["endpoints"]

        vm_a = get_vm(config, a)
        vm_b = get_vm(config, b)

        print(f"\n{a} <-> {b}")

        # A -> B
        try:
            output = ssh_command(
                vm_a,
                f"ping -c 5 -W 2 {quote(vm_b['host'])}",
            )

            for line in output.splitlines():
                if "packet loss" in line or "rtt " in line:
                    print(f"  {a} -> {b}: {line}")
        except Exception as e:
            print(f"  {a} -> {b}: ERROR: {e}")

        # B -> A
        try:
            output = ssh_command(
                vm_b,
                f"ping -c 5 -W 2 {quote(vm_a['host'])}",
            )

            for line in output.splitlines():
                if "packet loss" in line or "rtt " in line:
                    print(f"  {b} -> {a}: {line}")
        except Exception as e:
            print(f"  {b} -> {a}: ERROR: {e}")


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Configure bidirectional artificial network latency between Linux VMs using tc/netem. The VMs are defined in a YAML configuration file and must be accessible via SSH in non-interactive mode using a key. The SSH user must have passwordless sudo access."
        ),
        # Show default values for options with "--help" option.
        formatter_class=argparse.ArgumentDefaultsHelpFormatter,
    )

    parser.add_argument(
        "command",
        choices=["apply", "clear", "status", "test"],
        help=(
            "Operation to perform: apply latency configuration, clear tc configuration, show current tc status, or test configured links with ping."
        ),
    )

    parser.add_argument(
        "-c",
        "--config",
        dest="config_path",
        type=Path,
        default=Path("./config.yaml"),
        help="Path to the YAML configuration file.",
    )

    args = parser.parse_args()

    # Here, we only parse the configuration and call the respective command.
    try:
        config = yaml.safe_load(args.config_path.read_text())

        match args.command:
            case "apply":
                command_apply(config)
            case "clear":
                command_clear(config)
            case "status":
                command_status(config)
            case "test":
                command_test(config)
    except KeyboardInterrupt:
        print("Interrupted.")
        sys.exit(1)
    except Exception as e:
        print(f"ERROR: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
