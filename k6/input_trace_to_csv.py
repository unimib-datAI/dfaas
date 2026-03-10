#!/usr/bin/env bash
# SPDX-License-Identifier: AGPL-3.0-or-later.
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# This small Python script parses a JSON-formatted execution trace for a
# specific function and node, then converts it into a CSV file.
#
# The generated CSV saves the number of average requests per second (RPS) for
# each stage/iteration found in the trace.
#
# The input JSON file must follow this structure:
#
# {
#   "0": {
#     "0": [
#       722.9090101153404,
#       725.2641897188737,
#       728.2975171383529,
#       723.3259920611976,
#       721.2349287791351,
#       ...
#     ]
#   }
# }
#
# Where top-level keys are the function identifiers, second-level keys are not
# identifiers.
import json
import csv
import argparse


def main():
    parser = argparse.ArgumentParser(
        description="Extract from JSON the input trace of a specific function and node."
    )
    parser.add_argument("input_file", help="Path to the input JSON file.")
    parser.add_argument("output_file", help="Path to the output CSV file.")
    parser.add_argument("function_name", help="Function name.")
    parser.add_argument("node_name", help="Node name.")
    args = parser.parse_args()

    # Load JSON.
    with open(args.input_file, "r") as f:
        data = json.load(f)

    stage_values = data[args.function_name][args.node_name]

    rows = [[idx, value] for idx, value in enumerate(stage_values)]

    # Write CSV.
    with open(args.output_file, "w", newline="") as f:
        writer = csv.writer(f)
        writer.writerow(["stage", "avg_reqs_per_sec"])
        writer.writerows(rows)


if __name__ == "__main__":
    main()
