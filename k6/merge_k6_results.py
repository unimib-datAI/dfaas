#!/usr/bin/env python3
#
# Merge all k6_results_processed.csv files from the different nodes
# of an experiment into a single global CSV.
#
# Expected structure:
#
# experiment/
# └── k6/
#     ├── node_a/
#     │   └── k6_results_processed.csv
#     ├── node_b/
#     │   └── k6_results_processed.csv
#     └── global/
#         └── k6_results_processed.csv  <-- generated output
#
# The generated CSV contains all rows from all nodes and an additional
# "node" column identifying the source node.

import argparse
from pathlib import Path

import pandas as pd


def merge_experiment(exp_dir):
    # Merge all node CSV files for a single experiment.
    #
    # Example:
    # data/.../20260727_node_a_1/k6/node_a/k6_results_processed.csv

    k6_dir = exp_dir / "k6"
    global_dir = k6_dir / "global"

    # Find all node-specific processed k6 result files.
    # This matches:
    #   k6/node_a/k6_results_processed.csv
    #   k6/node_b/k6_results_processed.csv
    csv_files = sorted(k6_dir.glob("node_*/k6_results_processed.csv"))

    if not csv_files:
        print(f"[WARN] No CSV files found in {exp_dir}")
        return

    dfs = []

    for csv_file in csv_files:
        # Extract node name from the parent directory.
        # Example:
        # .../k6/node_a/k6_results_processed.csv -> node_a
        node = csv_file.parent.name

        print(f"  Reading {csv_file}")

        # Read the processed k6 results.
        df = pd.read_csv(csv_file)

        # Add the source node to each row. This allows grouping/filtering after
        # the merge.
        df["node"] = node

        dfs.append(df)

    # Combine all node dataframes into one dataframe.
    merged = pd.concat(dfs, ignore_index=True)

    # Make sure the global output directory exists.
    global_dir.mkdir(parents=True, exist_ok=True)

    output = global_dir / "k6_results_processed.csv"
    merged.to_csv(output, index=False)

    print(f"[DONE] {exp_dir.name}: {len(merged)} rows -> {output}")


def expand_experiments(paths):
    # Expand input paths into experiment directories.
    #
    # Supported inputs:
    #   1. A single experiment directory:
    #      data/.../20260727_node_a_1
    #
    #   2. A directory containing multiple experiments:
    #      data/20260727_1_rl_4_al_tests
    #
    #   3. A mix of both.

    experiments = []

    for path in paths:
        path = Path(path)

        if (path / "k6").exists():
            # If this is already an experiment directory, add it directly.
            experiments.append(path)
        elif path.is_dir():
            # Otherwise, search for experiment directories inside it.
            for exp_dir in sorted(path.iterdir()):
                if exp_dir.is_dir() and (exp_dir / "k6").exists():
                    experiments.append(exp_dir)
        else:
            print(f"[WARN] Ignoring invalid path: {path}")

    return experiments


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Merge k6_results_processed.csv files from all nodes "
            "into each experiment global directory."
        )
    )

    parser.add_argument(
        "experiments",
        nargs="+",
        help=("Experiment directories or directories containing multiple experiments."),
    )

    args = parser.parse_args()

    # Resolve all experiment directories.
    experiments = expand_experiments(args.experiments)

    print(f"Found {len(experiments)} experiments")

    # Merge each experiment independently.
    for exp_dir in experiments:
        merge_experiment(exp_dir)


if __name__ == "__main__":
    main()
