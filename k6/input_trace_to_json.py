#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later.
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# See --help message for more informatio about this script.

import json
import argparse
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(
        description=(
            "This script parses a JSON output trace generated from dfaas_input_rate.py (in DFaaS MARL) and outputs "
            "the JSON format required by the custom k6 script.\n\n"
            "The input JSON file must follow this structure:\n"
            '{"node_X": [722, 725, 728, ...], "nodeY": ..., ...}\n'
            "The output JSON will have the top-level key as the function"
            '(e.g., "0"), and second-level keys as node IDs (without the "node_"'
            "prefix)."
        )
    )
    parser.add_argument("input", type=Path, help="Path to the input JSON file.")
    parser.add_argument("output", type=Path, help="Path to the output JSON file.")
    parser.add_argument(
        "--function-name", type=str, default="0", help="Function name (e.g., '0')."
    )
    parser.add_argument(
        "--steps",
        type=int,
        default=None,
        help="Cut the trace after the given steps (if None all trace is kept).",
    )
    args = parser.parse_args()

    if args.steps is not None:
        assert args.steps > 0, "--steps must be a positive integer."

    data = json.loads(args.input.read_text())

    # Build new JSON format: { "function_name": { "X": [values] } }.
    out_json = {args.function_name: {}}
    for node_key, values in data.items():
        if node_key.startswith("node_"):
            node_id = node_key[len("node_") :]
        else:
            node_id = node_key

        # Optionally cut the values on a specific length.
        if args.steps is not None:
            values = values[: args.steps]

        out_json[args.function_name][node_id] = values

    args.output.write_text(json.dumps(out_json, indent=2) + "\n")


if __name__ == "__main__":
    main()
