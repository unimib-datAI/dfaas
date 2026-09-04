#!/usr/bin/env python3
#
# Merge the global k6 CSVs from multiple experiments into a single CSV.
# Adds an "rl_nodes" column extracted from each experiment directory name.
#
# Example:
#   20260820_node_f_g_d -> rl_nodes = "F G D"
#
# Assumes RL nodes are single letters at the end of the experiment name,
# following the "_node_" pattern.

import argparse
import re
from pathlib import Path
import pandas as pd


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("directory", help="Directory containing experiment directories")
    args = parser.parse_args()

    base = Path(args.directory)
    dfs = []

    for exp in sorted(base.iterdir()):
        csv = exp / "k6/global/k6_results_processed.csv"
        if not csv.exists():
            print(f"Missing: {csv}")
            continue

        # Assumes RL nodes are single letters at the end: *_node_a_b, *_node_f_g_d, etc.
        m = re.search(r"_node_([a-z](?:_[a-z])*)$", exp.name)
        if not m:
            print(f"Cannot extract RL nodes: {exp.name}")
            continue

        rl_nodes = " ".join(x.upper() for x in m.group(1).split("_"))
        print(f"Reading {csv} [RL nodes: {rl_nodes}]")

        df = pd.read_csv(csv)
        df["rl_nodes"] = rl_nodes
        dfs.append(df)

    if not dfs:
        raise SystemExit("No global CSV files found.")

    output = Path.cwd() / "k6_results_all_experiments.csv"
    pd.concat(dfs, ignore_index=True).to_csv(output, index=False)

    print(f"Done. {len(dfs)} experiments -> {output}")


if __name__ == "__main__":
    main()
