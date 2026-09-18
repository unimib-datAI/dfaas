# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import argparse
from pathlib import Path

import matplotlib.pyplot as plt
import polars as pl


def validate_nodes(df, expected_nodes=5):
    # Verify that the CSV contains the expected number of nodes.
    if "node" not in df.columns:
        raise ValueError("CSV does not contain 'node' column")

    nodes = sorted(df["node"].unique().to_list())

    if len(nodes) != expected_nodes:
        print(f"[WARN] Expected {expected_nodes} nodes, found {len(nodes)}: {nodes}")
        return False

    return True


def process_csv(input_csv, output_pdf, rl_strategy=False):
    print(f"Loading: {input_csv}")

    df = pl.read_csv(input_csv, null_values=[""])

    if rl_strategy:
        if "phase" not in df.columns:
            raise ValueError(
                "CSV does not contain 'phase' column required for RL strategy"
            )

        df = df.filter(pl.col("phase") == "rl_agent")
        print("Filtering phase == 'rl_agent'")

        if df.height == 0:
            raise ValueError("CSV is empty after filtering")

    if not validate_nodes(df):
        raise SystemExit(1)

    # Aggregate k6 results and prepare data for plotting.
    # Assumption: each k6 iteration represents a 60-second interval.
    df = (
        df.group_by(["iteration", "node"])
        .agg(
            [
                ((pl.col("http_status") == 200).mean() * 100).alias("success_rate"),
                (pl.len() / 60).alias("rps"),
            ]
        )
        .sort(["iteration", "node"])
    )

    success = df.pivot(values="success_rate", index="iteration", on="node")

    rps = df.pivot(values="rps", index="iteration", on="node").rename(
        {c: f"{c}_rps" for c in df["node"].unique().to_list()}
    )

    df = success.join(rps, on="iteration").sort("iteration")

    # Now we can start plotting.

    iterations = df["iteration"].to_list()

    nodes = [c for c in df.columns if c != "iteration" and not c.endswith("_rps")]

    fig, axes = plt.subplots(
        nrows=len(nodes),
        ncols=1,
        figsize=(18, 3.5 * len(nodes)),
        sharex=True,
    )

    if len(nodes) == 1:
        axes = [axes]

    for ax1, node in zip(axes, nodes):
        rps_col = f"{node}_rps"

        # Success rate.
        ax1.plot(
            iterations,
            df[node].to_list(),
            marker="o",
            color="tab:blue",
            label="Success %",
        )

        ax1.set(ylabel="Successful requests (%)", ylim=(0, 105), title=node)
        ax1.grid(True)

        # Requests/sec.
        if rps_col in df.columns:
            ax2 = ax1.twinx()

            ax2.plot(
                iterations,
                df[rps_col].to_list(),
                marker="x",
                linestyle="--",
                color="tab:orange",
                label="req/sec",
            )

            ax2.set_ylabel("Requests per second")

            lines1, labels1 = ax1.get_legend_handles_labels()
            lines2, labels2 = ax2.get_legend_handles_labels()

            ax1.legend(lines1 + lines2, labels1 + labels2, loc="best")

    axes[-1].set_xlabel("Iteration")

    fig.suptitle(
        "Success request rate and requests/sec per node (60s iterations)",
        y=1.01,
    )

    plt.tight_layout()
    fig.savefig(output_pdf, dpi=300, bbox_inches="tight")
    plt.close(fig)

    print(f"Saved plot: {output_pdf}")
    print()


def process_experiment(exp, rl_strategy=False):
    input_csv = exp / "k6" / "global" / "k6_results_processed.csv"
    output_pdf = exp / "k6" / "global" / "success_rate_rps_per_node.pdf"

    if not input_csv.exists():
        raise FileNotFoundError(f"Missing input CSV: {input_csv}")

    process_csv(input_csv, output_pdf, rl_strategy)


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Generate per-node k6 performance plots showing HTTP success "
            "rate and requests/sec over experiment iterations."
        ),
        epilog="""
The output plot has one subplot per DFaaS node. The X-axis shows
experiment iterations, while the two Y-axes show the successful request
percentage and requests per second received by each node.

WARNING: the RPS value assumes that each iteration represents a 60-second
measurement interval.

Running examples:

  uv run plot_success_rate_rps_per_node.py data/my_experiment

  uv run plot_success_rate_rps_per_node.py data/my_experiment --rl-strategy

  uv run plot_success_rate_rps_per_node.py --input-csv results.csv --output-pdf plot.pdf --rl-strategy
""",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )

    parser.add_argument(
        "experiments",
        type=Path,
        nargs="*",
        help="Experiment directories",
    )

    parser.add_argument("--input-csv", type=Path, help="Direct input CSV file")

    parser.add_argument("--output-pdf", type=Path, help="Output PDF plot file")

    parser.add_argument(
        "--rl-strategy",
        action="store_true",
        help="Enable this option only if the experiment used the RL Agent strategy",
    )

    args = parser.parse_args()

    # Direct CSV mode.
    if args.input_csv:
        if args.output_pdf is None:
            parser.error("--input-csv requires --output-pdf")

        process_csv(args.input_csv, args.output_pdf, args.rl_strategy)
        return

    # Experiment directory mode. The input CSV is expected at the standard
    # k6/global location inside the experiment directory.
    if not args.experiments:
        parser.error("provide experiment directories or use --input-csv")

    for exp in args.experiments:
        process_experiment(exp, args.rl_strategy)


if __name__ == "__main__":
    main()
