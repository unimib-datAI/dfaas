#!/usr/bin/env python3
#
# Run with --help flag for more information.
#
# Run with uv:
#
#   uv run python merge_all_k6_results.py <experiment_directory>
#
import argparse
import re
from pathlib import Path

import polars as pl


def extract_rl_nodes(exp_name):
    # First try the full experiment naming convention:
    #   20260820_node_a_b_c -> A B C
    m = re.search(r"_node_([a-z](?:_[a-z])*)$", exp_name)

    if m:
        nodes = m.group(1)
    else:
        # Otherwise assume the whole directory name contains only nodes:
        #   a -> A
        #   a_b -> A B
        if not re.fullmatch(r"[a-z](?:_[a-z])*", exp_name):
            return None

        nodes = exp_name

    return " ".join(x.upper() for x in nodes.split("_"))


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Merge the global k6 CSVs from multiple experiments into a single CSV. "
            "Adds an 'rl_nodes' column extracted from each experiment directory name."
        ),
        epilog="""
Examples:

  20260820_node_f_g_d -> rl_nodes = "F G D"
  a                   -> rl_nodes = "A"
  a_b                 -> rl_nodes = "A B"
  f_g_d               -> rl_nodes = "F G D"

The script assumes RL nodes are single letters: at the end of the experiment
name following the "_node_" pattern, or the entire experiment directory name
when no prefix exists.

The given directory must contain experiment directories:

    experiments/
    ├── 20260820_node_f_g_d/
    │   └── k6/
    │       └── global/
    │           └── k6_results_processed.csv
    ├── a_b/
    │   └── k6/
    │       └── global/
    │           └── k6_results_processed.csv

The merged CSV file is written as:

    <experiment_directory>/k6_results_all_experiments.csv

The original k6 results files (k6_results.csv.gz) must already have been
processed into k6/global/k6_results_processed.csv using k6_parser.py and
merge_k6_results.py scripts.
""",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )

    parser.add_argument(
        "directory",
        help="Directory containing experiment directories",
    )

    args = parser.parse_args()

    base = Path(args.directory)
    dfs = []

    for exp in sorted(base.iterdir()):
        csv = exp / "k6/global/k6_results_processed.csv"
        if not csv.exists():
            print(f"Missing: {csv}")
            continue

        rl_nodes = extract_rl_nodes(exp.name)

        if not rl_nodes:
            print(f"Cannot extract RL nodes: {exp.name}")
            continue

        print(f"Reading {csv} [RL nodes: {rl_nodes}]")

        df = pl.read_csv(csv)

        # Add the RL node configuration to every row.
        df = df.with_columns(pl.lit(rl_nodes).alias("rl_nodes"))

        dfs.append(df)

    if not dfs:
        raise SystemExit("No global CSV files found.")

    # Save output next to the experiment directories.
    output = base / "k6_results_all_experiments.csv"

    # Combine all experiments into one dataframe.
    # vertical_relaxed allows slightly different schemas between experiments.
    pl.concat(dfs, how="vertical_relaxed").write_csv(output)

    print(f"Done. {len(dfs)} experiments -> {output}")


if __name__ == "__main__":
    main()
