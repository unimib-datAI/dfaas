#!/usr/bin/env python
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

import pandas as pd


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
    df_iter = pd.read_csv(
        path,
        usecols=["metric_name", "extra_tags"],
        dtype={"metric_name": "string", "extra_tags": "string"},
        chunksize=50_000,
    )

    chunks = []
    for chunk in df_iter:
        chunks.append(chunk[chunk["metric_name"].eq("http_reqs")])

    if not chunks:
        return pd.DataFrame()

    df = pd.concat(chunks, ignore_index=True)

    df["stage"] = df["extra_tags"].str.extract(r"^stage=(.+)$")[0]
    df["stage"] = pd.to_numeric(df["stage"], errors="coerce").astype("Int64")
    df = df.dropna(subset=["stage"])

    if df.empty:
        return pd.DataFrame()

    # Merge stages 2-by-2: (0,1)->0, (2,3)->2, (4,5)->4, ...
    df["merged_stage"] = (df["stage"] // 2) * 2

    stage_counts = (
        df.groupby("merged_stage")
        .size()
        .rename("total_requests")
        .to_frame()
        .sort_index()
    )

    # avg req/s over merged stage duration (5s + 55s = 60s)
    # This is an assumption that the stages lasts 5s and 55s.
    stage_counts["avg_req_per_s"] = stage_counts["total_requests"] / 60

    # Re-index merged stages from 0..N-1 (instead of 0,2,4,...)
    stage_counts.index = range(len(stage_counts.index))
    stage_counts.index.name = "stage"

    return stage_counts


def run(paths):
    combined = None
    per_file_metric_cols = {"total_requests": [], "avg_req_per_s": []}

    for path in paths:
        prefix = _prefix_for_path(path)
        stage_counts = run_one(path)

        if stage_counts.empty:
            continue

        renamed = {
            "total_requests": prefix + "_total_requests",
            "avg_req_per_s": prefix + "_avg_req_per_s",
        }
        stage_counts = stage_counts.rename(columns=renamed)

        per_file_metric_cols["total_requests"].append(renamed["total_requests"])
        per_file_metric_cols["avg_req_per_s"].append(renamed["avg_req_per_s"])

        if combined is None:
            combined = stage_counts
        else:
            combined = combined.join(stage_counts, how="outer")

    if combined is None:
        return pd.DataFrame()

    combined = combined.sort_index()

    # Add generic aggregate columns (mean/std) across all files for each stage.
    # Note: std is sample standard deviation (pandas default, ddof=1); with one
    # file it will be NaN, which is expected.
    for metric, cols in per_file_metric_cols.items():
        if not cols:
            continue
        combined[f"mean_{metric}"] = combined[cols].mean(axis=1, skipna=True)
        combined[f"std_{metric}"] = combined[cols].std(axis=1, skipna=True)

    return combined


def main(argv=None):
    args = parse_args(argv)

    out = run(args.paths)

    # Show output table.
    print(out.to_string(index=True))

    # Save output CSV if requested.
    if args.output:
        output_path = Path(args.output)
        out.to_csv(output_path, index=True)
        print(f"Saved CSV to: {output_path.as_posix()}")

    return 0


if __name__ == "__main__":
    main()
