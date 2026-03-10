#!/usr/bin/env python
# SPDX-License-Identifier: AGPL-3.0-or-later.
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# This small script reads one or more k6 result files in CSV format and produces
# a summary table where each row represents a merged stage.
#
# Stages are merged in pairs as follows:
#   (0, 1) -> merged stage 0
#   (2, 3) -> merged stage 1
#   (4, 5) -> merged stage 2
#   ...
#
# For each input file, the script generates two columns per merged stage:
#   - <filename>_total_requests: total number of `http_reqs` in that merged
#   stage
#   - <filename>_avg_req_per_s: average requests per second for that stage
#
# After all per-file columns, the script also appends generic columns that
# aggregate across *all* input files for each stage (even if there is only one
# input file):
#   - mean_total_requests, std_total_requests
#   - mean_avg_req_per_s, std_avg_req_per_s
#
# The average requests per second are computed under the assumption that:
#   - stage 0 lasts 5 seconds
#   - stage 1 lasts 55 seconds
#   - therefore each merged stage (two consecutive stages) lasts 60 seconds
#
# The output table is printed to stdout. If the --output option is provided,
# the table is also saved as a CSV file at the specified path.
#
# Each output column name is prefixed with the corresponding input filename
# (without common extensions such as .csv or .csv.gz).
import argparse
from pathlib import Path

import polars as pl


def parse_args(argv=None):
    parser = argparse.ArgumentParser(
        description=(
            "Compute per-(merged) stage request rates from one or more k6 CSV files."
            "Output columns are prefixed by filename."
        )
    )
    parser.add_argument(
        "paths",
        nargs="+",
        help="One or more paths to k6 CSV files (e.g. k6_results_repeat_1.csv.gz)",
    )
    parser.add_argument(
        "--output",
        help="If set, save the output table as an uncompressed CSV at this path.",
    )
    return parser.parse_args(argv)


def _prefix_for_path(path):
    p = Path(path)
    name = p.name

    # handle common suffix combos like .csv.gz
    if name.endswith(".csv.gz"):
        return name[:-7]
    if name.endswith(".csv"):
        return name[:-4]
    if name.endswith(".gz"):
        return name[:-3]
    return name


def run_one(path):
    # Read CSV (only a subset of the columns).
    df = pl.scan_csv(path).select(["metric_name", "extra_tags"])

    # Get only http_reqs metric.
    df = df.filter(pl.col("metric_name") == "http_reqs")

    # Extract stage number from extra_tags using regex.
    df = df.with_columns(
        pl.col("extra_tags")
        .str.extract(r"^stage=(.+)$", 1)
        .cast(pl.Int64, strict=False)
        .alias("stage")
    )

    # Drop rows where stage extraction failed.
    df = df.filter(pl.col("stage").is_not_null())

    # Execute query and get the result.
    df = df.collect()
    if df.is_empty():
        return pl.DataFrame()

    # Merge stages 2-by-2: (0,1)->0, (2,3)->2, (4,5)->4, ...
    df = df.with_columns(((pl.col("stage") // 2) * 2).alias("merged_stage"))

    # Group by merged_stage and count.
    stage_counts = (
        df.group_by("merged_stage")
        .agg(pl.len().alias("total_requests"))
        .sort("merged_stage")
    )

    # avg req/s over merged stage duration (5s + 55s = 60s)
    # This is an assumption that the stages lasts 5s and 55s.
    stage_counts = stage_counts.with_columns(
        (pl.col("total_requests") / 60).alias("avg_req_per_s")
    )

    # Re-index merged stages from 0..N-1 (instead of 0,2,4,...)
    stage_counts = stage_counts.with_columns(
        pl.int_range(0, pl.len()).alias("stage")
    ).select(["stage", "total_requests", "avg_req_per_s"])

    return stage_counts


def run(paths):
    all_stage_dfs = []
    per_file_metric_cols = {"total_requests": [], "avg_req_per_s": []}

    for path in paths:
        prefix = _prefix_for_path(path)
        stage_counts = run_one(path)

        if stage_counts.is_empty():
            continue

        # Rename columns with file prefix.
        renamed = {
            "total_requests": prefix + "_total_requests",
            "avg_req_per_s": prefix + "_avg_req_per_s",
        }
        stage_counts = stage_counts.rename(renamed)

        per_file_metric_cols["total_requests"].append(renamed["total_requests"])
        per_file_metric_cols["avg_req_per_s"].append(renamed["avg_req_per_s"])

        all_stage_dfs.append(stage_counts)

    if not all_stage_dfs:
        return pl.DataFrame()

    # Start with the first dataframe
    combined = all_stage_dfs[0]

    # Join the rest one by one using 'full' join
    for stage_df in all_stage_dfs[1:]:
        combined = combined.join(stage_df, on="stage", how="full", coalesce=True)

    combined = combined.sort("stage")

    # Add generic aggregate columns (mean/std) across all files for each stage.
    # Note: std is sample standard deviation (ddof=1); with one file it will be null.
    for metric, cols in per_file_metric_cols.items():
        if not cols:
            continue
        combined = combined.with_columns(
            [
                pl.mean_horizontal(cols).alias(f"mean_{metric}"),
                pl.concat_list(cols).list.std(ddof=1).alias(f"std_{metric}"),
            ]
        )

    return combined


def main(argv=None):
    args = parse_args(argv)

    out = run(args.paths)

    # Show full table.
    out.show(limit=None)

    # Save output CSV if requested.
    if args.output:
        out.write_csv(args.output)
        print(f"Saved CSV to: {args.output}")


if __name__ == "__main__":
    main()
