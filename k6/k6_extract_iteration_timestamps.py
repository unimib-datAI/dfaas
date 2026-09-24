# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# This script takes as input a processed k6 CSV file for a single node of an
# experiment and outputs a CSV containing information for each registered
# iteration: start and end timestamps, and durations in seconds (redundant but
# useful).

import argparse
import sys
from pathlib import Path

import polars as pl

DEFAULT_ITERATION_DURATION_S = 60


def process(input_csv, output_csv, iteration_duration_s):
    df = pl.read_csv(input_csv)
    print(f"Input CSV: {input_csv}")

    # Reconstruct request start time.
    # k6 timestamps and http_req_duration are expressed in seconds.
    df = df.with_columns(
        (pl.col("timestamp") - pl.col("http_req_duration")).alias(
            "request_start_s"
        )
    )

    # Get the first actual request start time of the test.
    test_start_s = df.select(pl.col("request_start_s").min()).item()

    result = (
        df.group_by("iteration")
        .agg(pl.len().alias("requests"))
        .with_columns(
            (
                test_start_s
                + pl.col("iteration") * iteration_duration_s
            ).alias("start_timestamp_s"),
            (
                test_start_s
                + (pl.col("iteration") + 1) * iteration_duration_s
            ).alias("end_timestamp_s"),
        )
        .with_columns(
            pl.col("start_timestamp_s").round().cast(pl.Int64),
            pl.col("end_timestamp_s").round().cast(pl.Int64),
            pl.lit(iteration_duration_s)
            .cast(pl.Int64)
            .alias("duration_s"),
        )
        .select(
            "iteration",
            "start_timestamp_s",
            "end_timestamp_s",
            "duration_s",
        )
        .sort("iteration")
    )

    output_csv.parent.mkdir(parents=True, exist_ok=True)
    result.write_csv(output_csv)

    print(f"Iterations found: {result.height}")
    print(f"Output CSV: {output_csv}")


def main():
    parser = argparse.ArgumentParser(
        description="Extract k6 iteration timestamps from HTTP request results"
    )

    parser.add_argument(
        "--input",
        type=Path,
        required=True,
        help="Path to input CSV file",
    )

    parser.add_argument(
        "--output",
        type=Path,
        required=True,
        help="Path to output CSV file",
    )

    parser.add_argument(
        "--duration",
        type=int,
        default=DEFAULT_ITERATION_DURATION_S,
        help=f"Iteration duration in seconds (default: {DEFAULT_ITERATION_DURATION_S})",
    )

    args = parser.parse_args()

    if args.duration <= 0:
        print(f"Got {args.duration} seconds as duration, must be positive!")
        sys.exit(1)

    process(args.input.resolve(), args.output.resolve(), args.duration)


if __name__ == "__main__":
    main()
