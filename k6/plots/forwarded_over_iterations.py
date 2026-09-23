# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import argparse
from pathlib import Path

import matplotlib.pyplot as plt
import polars as pl
from matplotlib.ticker import FixedLocator

ITERATION_DURATION = 60


def plot(node, nodes, forwards, rejections, incoming, output_pdf):
    iterations = sorted(set(incoming["iteration"].to_list()))
    x = list(range(len(iterations)))

    fig, axes = plt.subplots(5, 1, figsize=(16, 14), sharex=True)

    def add_rps_line(ax, current_node):
        values = incoming.filter(pl.col("node") == current_node).select(
            ["iteration", "rps"]
        )

        rps = dict(values.iter_rows())

        line_values = [rps.get(iteration, 0) for iteration in iterations]

        ax.plot(
            x,
            line_values,
            marker="x",
            linestyle="--",
            color="black",
            label="Incoming rate",
        )

        return line_values

    for ax, current_node in zip(axes, nodes):
        if current_node == node:
            data = forwards.filter(pl.col("iteration").is_in(iterations))

            recipients = sorted(data["forwarded_to"].unique().to_list())

            bottom = [0.0] * len(iterations)
            max_bar = 0

            for recipient in recipients:
                values = data.filter(pl.col("forwarded_to") == recipient).select(
                    ["iteration", "rps"]
                )

                values = dict(values.iter_rows())

                bar_values = [values.get(iteration, 0) for iteration in iterations]

                max_bar = max(
                    max_bar,
                    max(a + b for a, b in zip(bottom, bar_values)),
                )

                ax.bar(x, bar_values, bottom=bottom, label=recipient)

                bottom = [a + b for a, b in zip(bottom, bar_values)]

            line_values = add_rps_line(ax, current_node)

            ax.set_title(f"Forwarded requests on {current_node}")

        else:
            data = rejections.filter(pl.col("node") == current_node)

            values = dict(data.select(["iteration", "rps"]).iter_rows())

            bar_values = [values.get(iteration, 0) for iteration in iterations]

            ax.bar(x, bar_values, label="Local rejection", color="red")

            max_bar = max(bar_values, default=0)

            line_values = add_rps_line(ax, current_node)

            ax.set_title(f"Local rejections on {current_node}")

        ymax = max(
            max_bar,
            max(line_values),
            1,
        )

        ax.set_ylim(0, ymax * 1.15)
        ax.set_ylabel("Requests/s")
        ax.grid(axis="y", linestyle="--", alpha=0.3)

        handles, labels = ax.get_legend_handles_labels()

        ax.legend(
            handles,
            labels,
            loc="upper center",
            bbox_to_anchor=(0.5, -0.18),
            ncol=4,
            fontsize=8,
        )

    # Show tick on X-axis from 0 to 100, every 5 ticks.
    ticks = sorted(set(range(0, len(iterations), 5)) | {len(iterations) - 1})
    for ax in axes:
        ax.xaxis.set_major_locator(FixedLocator(ticks))
        ax.set_xticklabels([iterations[i] for i in ticks], rotation=90, fontsize=8)

        # Make sure the tick labels are shown on each sub-plots!
        ax.tick_params(axis="x", labelbottom=True)

    # But show the X-axis name only on the last plot.
    axes[-1].set_xlabel("Iteration")

    plt.tight_layout()

    fig.savefig(output_pdf, dpi=300, bbox_inches="tight")
    plt.close(fig)

    print(f"Saved plot: {output_pdf}")
    print()


def process_csv(input_csv, output_pdf, node, output_csv=None):
    print(f"Loading: {input_csv}")

    df = pl.read_csv(input_csv)

    if "node" not in df.columns:
        raise ValueError("CSV does not contain 'node' column")

    if "phase" not in df.columns:
        raise ValueError("CSV does not contain 'phase' column")

    df = df.filter(pl.col("phase") == "rl_agent")

    if df.height == 0:
        raise ValueError("CSV is empty after filtering phase == 'rl_agent'")

    nodes = sorted(df["node"].unique().to_list())

    if len(nodes) > 5:
        raise ValueError(f"Expected at most 5 agents, found {len(nodes)}: {nodes}")

    if node not in nodes:
        raise ValueError(f"Node {node!r} not found. Available: {nodes}")

    # Create a mapping from nodes' ID to node's name.
    mapping = dict(
        df.filter(pl.col("dfaas_node_id").is_not_null())
        .select(["dfaas_node_id", "node"])
        .unique()
        .iter_rows()
    )

    forwarded = pl.col("dfaas_forwarded_to").is_not_null()

    local_rejection = (
        ~forwarded & (pl.col("http_status") != 200) & (pl.col("http_status") != 403)
    )

    incoming = (
        df.group_by(["node", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
    )

    forwards = (
        df.filter((pl.col("node") == node) & forwarded)
        .with_columns(
            pl.col("dfaas_forwarded_to").replace(mapping).alias("forwarded_to")
        )
        .group_by(["iteration", "forwarded_to"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .sort(["iteration", "forwarded_to"])
    )

    rejections = (
        df.filter(local_rejection)
        .group_by(["node", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .sort(["node", "iteration"])
    )

    if output_csv:
        forwards.write_csv(output_csv)
        print(f"Saved CSV: {output_csv}")

    plot(node, nodes, forwards, rejections, incoming, output_pdf)


def process_experiment(exp, node):
    input_csv = exp / "k6" / "global" / "k6_results_processed.csv"
    output_pdf = exp / "k6" / "global" / f"{node}_forwarding_over_iterations.pdf"
    output_csv = exp / "k6" / "global" / f"{node}_forwarding_over_iterations.csv"

    if not input_csv.exists():
        raise FileNotFoundError(f"Missing input CSV: {input_csv}")

    process_csv(input_csv, output_pdf, node, output_csv)


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Plot request forwarding and local rejection behaviour "
            "for DFaaS agents during the RL strategy phase. The plot "
            "shows forwarded requests for the selected RL agent and "
            "local rejections for the remaining agents. The script "
            "supports direct CSV mode or experiment mode. In direct "
            "CSV mode, --input-csv and --output-pdf are required. "
            "In experiment mode, one or more experiment directories "
            "are processed sequentially using predefined input/output "
            "paths."
        )
    )

    parser.add_argument(
        "experiments",
        type=Path,
        nargs="*",
        help=(
            "Experiment directories. Multiple experiments are "
            "processed sequentially. Input CSV and output PDF paths "
            "are generated automatically."
        ),
    )

    parser.add_argument(
        "--input-csv",
        type=Path,
        help=("Input CSV file. Required when no experiment directory is provided."),
    )

    parser.add_argument(
        "--output-pdf",
        type=Path,
        help=("Output PDF file. Required when no experiment directory is provided."),
    )

    parser.add_argument(
        "--node",
        required=True,
        help=(
            "RL agent node to analyze. The node must participate in "
            "the experiment and use the RL strategy phase."
        ),
    )

    args = parser.parse_args()

    print("WARNING: This script assumes the experiment ran with RL strategy!")
    print()

    if args.experiments:
        if args.input_csv or args.output_pdf:
            parser.error(
                "--input-csv and --output-pdf cannot be used with experiment directories"
            )

        for exp in args.experiments:
            process_experiment(exp, args.node)

        return

    if args.input_csv is None or args.output_pdf is None:
        parser.error(
            "--input-csv and --output-pdf are required when no experiment directory is provided"
        )

    process_csv(args.input_csv, args.output_pdf, args.node)


if __name__ == "__main__":
    main()
