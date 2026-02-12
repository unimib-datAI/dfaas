#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later.
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# A small CLI utility to extract and print the start and end date of a k6
# experiments, used to later extract Prometheus metrics from DFaaS nodes.
#
# Run the script with the --help flag for more details.

import argparse
import sys
from pathlib import Path

import pandas as pd


def main():
    parser = argparse.ArgumentParser(
        description="Prints start and end time from a k6 CSV metrics file."
    )
    parser.add_argument(
        "metrics_file",
        type=Path,
        help="Path to the CSV metrics file (e.g., k6_results.csv.gz)",
    )
    args = parser.parse_args()

    df = pd.read_csv(args.metrics_file, usecols=["timestamp"])

    df["dt"] = pd.to_datetime(df["timestamp"], unit="s", errors="coerce")
    if df["dt"].isnull().all():
        print("Could not find any valid timestamp data.", file=sys.stderr)
        return 1

    start = df["dt"].min()
    end = df["dt"].max()

    fmt = "%Y-%m-%dT%H:%M:%S"
    print("Start:", start.strftime(fmt))
    print("End:", end.strftime(fmt))


if __name__ == "__main__":
    sys.exit(main())
