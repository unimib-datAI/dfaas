#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later.
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# Run the script with the --help flag for more details.
#!/usr/bin/env python3
import argparse
import csv
import subprocess
import time
import re
import random
from datetime import datetime

# Internal function database with their test bodies
FUNCTION_DATABASE = {"figlet": "Hello World"}


def delete_function_pods(function_name):
    """Delete all pods related to the function using kubectl."""
    try:
        cmd = [
            "sudo",
            "kubectl",
            "delete",
            "pod",
            "-l",
            f"faas_function={function_name}",
            "--wait=false",
        ]
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
        return result.returncode == 0
    except subprocess.TimeoutExpired:
        print("  Warning: kubectl delete command timed out")
        return False
    except Exception as e:
        print(f"  Error deleting pods: {e}")
        return False


def wait_for_pods_terminating(function_name, max_wait=30):
    """Wait until pods are in Terminating state."""
    start_time = time.time()

    while time.time() - start_time < max_wait:
        try:
            cmd = [
                "sudo",
                "kubectl",
                "get",
                "pods",
                "-l",
                f"faas_function={function_name}",
                "--no-headers",
            ]
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=10)

            output = result.stdout.strip()

            # Check if there are any pods
            if not output:
                # No pods found - they might have been deleted too quickly
                elapsed = time.time() - start_time
                print(f"  Pods already deleted after {elapsed:.2f}s")
                return True

            # Check if any pod is in Terminating state
            lines = output.split("\n")
            for line in lines:
                if "Terminating" in line:
                    elapsed = time.time() - start_time
                    print(f"  Pod(s) in Terminating state after {elapsed:.2f}s")
                    return True

            time.sleep(0.1)

        except Exception as e:
            print(f"  Error checking pod status: {e}")
            time.sleep(0.1)

    print(
        f"  Warning: Pods not in Terminating state after {max_wait}s, proceeding anyway"
    )
    return False


def invoke_function(endpoint, function_name, body):
    """Invoke the function using curl and extract metrics."""
    url = f"http://{endpoint}/function/{function_name}"

    cmd = [
        "curl",
        "--http1.1",
        "--no-keepalive",
        "-s",
        "-w",
        "cold_start_service_time_s=%{time_total} time_to_first_byte_s=%{time_starttransfer}\n",
        url,
        "-d",
        body,
        "-i",
    ]

    try:
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=120)
        output = result.stdout

        # Extract curl timing metrics
        cold_start_match = re.search(r"cold_start_service_time_s=([\d.]+)", output)
        ttfb_match = re.search(r"time_to_first_byte_s=([\d.]+)", output)

        cold_start_time = float(cold_start_match.group(1)) if cold_start_match else None
        ttfb = float(ttfb_match.group(1)) if ttfb_match else None

        # Extract x-duration-seconds header
        x_duration_match = re.search(
            r"x-duration-seconds:\s*([\d.]+)", output, re.IGNORECASE
        )
        x_duration = float(x_duration_match.group(1)) if x_duration_match else None

        return cold_start_time, ttfb, x_duration

    except subprocess.TimeoutExpired:
        print("  Error: curl command timed out")
        return None, None, None
    except Exception as e:
        print(f"  Error invoking function: {e}")
        return None, None, None


def run_benchmark(endpoint, function_name, cycles, output_file):
    """Run the cold start benchmark."""
    # Get body from database, default to "Hello World" if not found
    body = FUNCTION_DATABASE.get(function_name, "Hello World")

    print(f"Starting benchmark for function '{function_name}'")
    print(f"Endpoint: {endpoint}")
    print(f"Cycles: {cycles}")
    print(f"Output file: {output_file}")
    print(f"Request body: {body}")
    print("-" * 60)

    # Open CSV file and write header
    with open(output_file, "w", newline="") as csvfile:
        writer = csv.writer(csvfile)
        writer.writerow(
            [
                "timestamp",
                "function_name",
                "cold_start_service_time_s",
                "time_to_first_byte_s",
                "service_time_s",
            ]
        )

        # Run benchmark cycles
        for cycle in range(1, cycles + 1):
            print(f"\nCycle {cycle}/{cycles}")

            # Step 1: Delete function pods
            print(f"  Deleting pods for {function_name}...")
            delete_function_pods(function_name)

            # Step 2: Wait until pods are in Terminating state
            print(f"  Waiting for pods to be in Terminating state...")
            wait_for_pods_terminating(function_name)

            # Step 3: Immediately invoke the function
            print(f"  Invoking function...")
            cold_start, ttfb, x_duration = invoke_function(
                endpoint, function_name, body
            )

            if cold_start is None:
                print(f"  Warning: Failed to get metrics for cycle {cycle}")
                timestamp = datetime.now().isoformat()
                writer.writerow([timestamp, function_name, "", "", ""])
            else:
                # Step 4 & 5: Save metrics to CSV
                timestamp = datetime.now().isoformat()
                writer.writerow(
                    [
                        timestamp,
                        function_name,
                        f"{cold_start:.6f}",
                        f"{ttfb:.6f}",
                        f"{x_duration:.6f}" if x_duration is not None else "",
                    ]
                )

                print(f"  Metrics:")
                print(f"    Cold start service time: {cold_start:.6f}s")
                print(f"    Time to first byte: {ttfb:.6f}s")
                print(
                    f"    Service time (x-duration): {x_duration:.6f}s"
                    if x_duration
                    else "    Service time: N/A"
                )

            # Flush to ensure data is written
            csvfile.flush()

            # Step 6: Random wait before next cycle (if not the last cycle)
            if cycle < cycles:
                wait_time = random.uniform(0, 1)
                print(f"  Waiting {wait_time:.2f}s before next cycle...")
                time.sleep(wait_time)

    print("\n" + "=" * 60)
    print(f"Benchmark complete! Results saved to {output_file}")


def main():
    parser = argparse.ArgumentParser(
        description="Cold Start Benchmark Tool for OpenFaaS Functions",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s 10.0.2.38:30080 figlet
  %(prog)s 10.0.2.38:30080 figlet -c 20 -o custom_output.csv
        """,
    )

    parser.add_argument("endpoint", help="Endpoint address (e.g., 10.0.2.38:30080)")

    parser.add_argument("function", help="Function name to benchmark (e.g., figlet)")

    parser.add_argument(
        "-c",
        "--cycles",
        type=int,
        default=10,
        help="Number of benchmark cycles to run (default: 10)",
    )

    parser.add_argument(
        "-o",
        "--output",
        default="cold_service_time.csv",
        help="Output CSV file path (default: cold_service_time.csv)",
    )

    args = parser.parse_args()

    # Validate cycles
    if args.cycles <= 0:
        parser.error("Cycles must be a positive integer")

    # Run the benchmark
    run_benchmark(args.endpoint, args.function, args.cycles, args.output)


if __name__ == "__main__":
    main()
