#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later.
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# A small CLI utility to print stats for traces given as JSON format.
#
# Run the script with the --help flag for more details.

import json
import argparse
from pathlib import Path

import pandas as pd


def main():
    parser = argparse.ArgumentParser(
        description="Print stats data about traces from JSON file"
    )
    parser.add_argument("traces", help="traces path in JSON format")
    args = parser.parse_args()

    data = json.loads(Path(args.traces).read_text())

    percentiles = [0.25, 0.5, 0.75, 0.8, 0.9, 0.95]

    for function, nodes in data.items():
        stats_per_node = {}
        for node, values in nodes.items():
            s = pd.Series(values)
            stats_per_node[f"Node {node}"] = s.describe(percentiles=percentiles)

        stats_df = pd.DataFrame(stats_per_node)
        print(f"Function: {function!r}")
        print(stats_df.round(2))
        print()


if __name__ == "__main__":
    main()
