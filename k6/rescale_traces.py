#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later.
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# A small CLI utility to rescale input requests traces.
#
# Run the script with the --help flag for more details.

import json
import argparse
from pathlib import Path


def find_min_max(obj, cur_min=None, cur_max=None):
    """Recursively find the minimum and maximum numeric values in a nested structure."""
    if isinstance(obj, dict):
        for v in obj.values():
            cur_min, cur_max = find_min_max(v, cur_min, cur_max)
    elif isinstance(obj, list):
        for v in obj:
            cur_min, cur_max = find_min_max(v, cur_min, cur_max)
    elif isinstance(obj, (int, float)):
        if cur_min is None or obj < cur_min:
            cur_min = obj
        if cur_max is None or obj > cur_max:
            cur_max = obj
    return cur_min, cur_max


def scale_numbers(obj, old_min, old_max, new_min=0, new_max=800, power=1.0):
    """Recursively scale all numbers in a nested structure with power transformation."""
    if isinstance(obj, dict):
        return {
            k: scale_numbers(v, old_min, old_max, new_min, new_max, power)
            for k, v in obj.items()
        }
    elif isinstance(obj, list):
        return [
            scale_numbers(v, old_min, old_max, new_min, new_max, power) for v in obj
        ]
    elif isinstance(obj, (int, float)):
        if old_max != old_min:
            norm = (obj - old_min) / (old_max - old_min)
            norm_power = pow(norm, power)
            return new_min + (new_max - new_min) * norm_power
        else:
            return float(new_min)
    else:
        return obj


def main():
    parser = argparse.ArgumentParser(
        description="Scale all traces of a JSON file to a new range using min-max and power scaling.",
        epilog="If --power=1.0, scaling acts as linear scaling.",
    )
    parser.add_argument("input_file", help="Input JSON file")
    parser.add_argument("output_file", help="Output JSON file")
    parser.add_argument(
        "--min", type=float, default=0, help="New minimum value (default: 0)"
    )
    parser.add_argument(
        "--max", type=float, default=800, help="New maximum value (default: 800)"
    )
    parser.add_argument(
        "--power", type=float, default=1.0, help="Power for scaling (default: 1.0)"
    )
    args = parser.parse_args()

    input_path = Path(args.input_file)
    output_path = Path(args.output_file)

    with input_path.open("r") as fin:
        data = json.load(fin)

    old_min, old_max = find_min_max(data)
    scaled_data = scale_numbers(data, old_min, old_max, args.min, args.max, args.power)

    with output_path.open("w") as fout:
        json.dump(scaled_data, fout, indent=2)


if __name__ == "__main__":
    main()
