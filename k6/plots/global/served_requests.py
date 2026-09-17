# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import argparse
from pathlib import Path

import matplotlib.pyplot as plt
import polars as pl


def classify_outcome(df):
    """
    Classify each incoming request based on its final outcome.

    Outcomes:
      - local_success
      - forward_success
      - agent_reject
      - local_reject
      - forward_reject
    """
    return df.with_columns(
        pl.when(pl.col("dfaas_forwarded_to").is_null() & (pl.col("http_status") == 200))
        .then(pl.lit("local_success"))
        .when(
            pl.col("dfaas_forwarded_to").is_not_null() & (pl.col("http_status") == 200)
        )
        .then(pl.lit("forward_success"))
        .when(pl.col("dfaas_forwarded_to").is_null() & (pl.col("http_status") == 503))
        .then(pl.lit("agent_reject"))
        .when(
            pl.col("dfaas_forwarded_to").is_null()
            & (pl.col("http_status") != 200)
            & (pl.col("http_status") != 503)
        )
        .then(pl.lit("local_reject"))
        .when(
            pl.col("dfaas_forwarded_to").is_not_null() & (pl.col("http_status") != 200)
        )
        .then(pl.lit("forward_reject"))
        .otherwise(pl.lit("unknown"))
        .alias("outcome")
    )


def aggregate(df):
    """
    Aggregate request outcomes per DFaaS node.

    Returns:
      - total_requests
      - percentage distribution of each outcome
    """
    totals = df.group_by("node").len().rename({"len": "total_requests"})
    counts = df.group_by(["node", "outcome"]).len().rename({"len": "requests"})

    return (
        counts.join(totals, on="node")
        .with_columns(
            (pl.col("requests") / pl.col("total_requests") * 100).alias("percentage")
        )
        .pivot(
            values="percentage",
            index="node",
            on="outcome",
            aggregate_function="first",
        )
        .fill_null(0)
        .join(totals, on="node")
    )


def plot(df, output):
    """
    Generate a 100% stacked bar chart showing request outcomes per node.
    """
    categories = [
        ("local_success", "Local success", "green"),
        ("forward_success", "Forward success", "gold"),
        ("agent_reject", "Agent reject", "blue"),
        ("local_reject", "Local reject", "red"),
        ("forward_reject", "Forward reject", "pink"),
    ]

    nodes = df["node"].to_list()

    fig, ax = plt.subplots(figsize=(8, 4))

    bottom = [0] * len(nodes)

    # Draw stacked bars for every outcome category
    for column, label, color in categories:
        values = df[column].to_list() if column in df.columns else [0] * len(nodes)

        ax.bar(nodes, values, bottom=bottom, label=label, color=color, width=0.65)

        bottom = [b + v for b, v in zip(bottom, values)]

    # Show absolute request count above each node
    for idx, total in enumerate(df["total_requests"].to_list()):
        ax.text(idx, 102, f"n={total}", ha="center", va="bottom", fontsize=8)

    ax.set(
        ylabel="Requests (%)",
        xlabel="DFaaS node",
        ylim=(0, 110),
        title="Incoming request outcome distribution per DFaaS node",
    )

    ax.grid(axis="y", linestyle="--", alpha=0.3)
    ax.legend(loc="upper center", bbox_to_anchor=(0.5, -0.18), ncol=3, fontsize=8)

    plt.tight_layout()

    fig.savefig(output, bbox_inches="tight")

    plt.close(fig)


def process_csv(input_csv, output_csv, output_pdf):
    """
    Process a single input CSV file.
    """
    print(f"Loading: {input_csv}")

    df = pl.read_csv(input_csv, null_values=[""])

    # Classify request outcomes
    df = classify_outcome(df)

    # Aggregate outcomes per node
    aggregated = aggregate(df)

    ordered_columns = [
        "node",
        "total_requests",
        "local_success",
        "forward_success",
        "agent_reject",
        "local_reject",
        "forward_reject",
    ]

    aggregated = aggregated.select(
        [c for c in ordered_columns if c in aggregated.columns]
    )
    aggregated = aggregated.sort("node")

    # Save outputs
    aggregated.write_csv(output_csv)
    plot(aggregated, output_pdf)

    print(f"Saved CSV: {output_csv}")
    print(f"Saved PDF: {output_pdf}")
    print()


def process_experiment(exp):
    """
    Process one experiment directory.
    """
    input_csv = exp / "k6" / "global" / "k6_results_processed.csv"

    output_dir = exp / "k6" / "global"
    output_csv = output_dir / "request_outcome_by_node.csv"
    output_pdf = output_dir / "request_outcome_by_node.pdf"

    process_csv(input_csv, output_csv, output_pdf)


def main():
    """
    Command-line entry point.

    Modes:

      Existing experiment mode:
        script.py experiment_directory

      Direct CSV mode:
        script.py --input-csv input.csv --output-csv output.csv --output-pdf plot.pdf
    """
    parser = argparse.ArgumentParser(
        description="Plot incoming request outcomes per DFaaS node"
    )

    parser.add_argument(
        "experiments",
        type=Path,
        nargs="*",
        help="Experiment directories",
    )

    parser.add_argument(
        "--input-csv",
        type=Path,
        help="Direct input CSV file instead of an experiment directory",
    )

    parser.add_argument(
        "--output-csv",
        type=Path,
        help="Output CSV file (required with --input-csv)",
    )

    parser.add_argument(
        "--output-pdf",
        type=Path,
        help="Output PDF plot file (required with --input-csv)",
    )

    args = parser.parse_args()

    # Direct CSV mode
    if args.input_csv:
        if args.output_csv is None or args.output_pdf is None:
            parser.error("--input-csv requires both --output-csv and --output-pdf")

        process_csv(args.input_csv, args.output_csv, args.output_pdf)
        return

    # Experiment directory mode
    if not args.experiments:
        parser.error("provide experiment directories or use --input-csv")

    for exp in args.experiments:
        process_experiment(exp)


if __name__ == "__main__":
    main()
