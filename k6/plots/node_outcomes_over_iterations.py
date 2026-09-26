# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import argparse
from pathlib import Path

import matplotlib.pyplot as plt
import polars as pl
from matplotlib.ticker import MaxNLocator, PercentFormatter


def process_csv(input_csv, output_pdf, node, output_csv=None):
    print(f"Loading: {input_csv}")

    df = pl.read_csv(input_csv)

    if "node" not in df.columns:
        raise ValueError("CSV does not contain 'node' column")

    nodes = sorted(df["node"].unique().to_list())

    if node not in nodes:
        raise ValueError(f"Node {node!r} not found. Available: {nodes}")

    df = df.filter(pl.col("node") == node)

    if df.height == 0:
        raise ValueError("CSV is empty after filtering")

    # Outcome classification.
    forwarded = pl.col("dfaas_forwarded_to").is_not_null()
    success = pl.col("http_status") == 200
    agent_reject = pl.col("http_status") == 403

    df = (
        df.group_by("iteration")
        .agg(
            [
                ((~forwarded & success).mean() * 100).round(1).alias("local_success_%"),
                ((~forwarded & agent_reject).mean() * 100)
                .round(1)
                .alias("agent_reject_%"),
                ((~forwarded & ~success & ~agent_reject).mean() * 100)
                .round(1)
                .alias("local_fail_%"),
                ((forwarded & success).mean() * 100)
                .round(1)
                .alias("forward_success_%"),
                ((forwarded & ~success).mean() * 100).round(1).alias("forward_fail_%"),
                (pl.len() / 60).round(1).alias("rps"),
            ]
        )
        .sort("iteration")
    )

    if output_csv:
        df.write_csv(output_csv)
        print(f"Saved CSV: {output_csv}")

    plot(df, node, output_pdf)


def plot(df, node, output_pdf):
    iterations = df["iteration"].to_list()

    local_success = df["local_success_%"].to_list()
    agent_reject = df["agent_reject_%"].to_list()
    local_fail = df["local_fail_%"].to_list()
    forward_success = df["forward_success_%"].to_list()
    forward_fail = df["forward_fail_%"].to_list()
    rps = df["rps"].to_list()

    x = range(len(iterations))

    def draw_plot(ax, scale_to_rps=False):
        ls = local_success.copy()
        fs = forward_success.copy()
        ar = agent_reject.copy()
        lf = local_fail.copy()
        ff = forward_fail.copy()

        if scale_to_rps:
            totals = [a + b + c + d + e for a, b, c, d, e in zip(ls, fs, ar, lf, ff)]

            scales = [r / t if t > 0 else 1.0 for r, t in zip(rps, totals)]

            ls = [v * s for v, s in zip(ls, scales)]
            fs = [v * s for v, s in zip(fs, scales)]
            ar = [v * s for v, s in zip(ar, scales)]
            lf = [v * s for v, s in zip(lf, scales)]
            ff = [v * s for v, s in zip(ff, scales)]

        # Order:
        # Local success
        # Fwd success
        # Agent reject
        # Local reject
        # Fwd reject
        ax.bar(x, ls, label="Local success", color="green")

        bottom = ls

        ax.bar(
            x,
            fs,
            bottom=bottom,
            label="Fwd success",
            color="gold",
        )

        bottom = [a + b for a, b in zip(bottom, fs)]

        ax.bar(
            x,
            ar,
            bottom=bottom,
            label="Agent reject",
            color="blue",
        )

        bottom = [a + b for a, b in zip(bottom, ar)]

        ax.bar(
            x,
            lf,
            bottom=bottom,
            label="Local reject",
            color="red",
        )

        bottom = [a + b for a, b in zip(bottom, lf)]

        ax.bar(
            x,
            ff,
            bottom=bottom,
            label="Fwd reject",
            color="pink",
        )

        if scale_to_rps:
            ax.plot(
                list(x),
                rps,
                marker="x",
                linestyle="--",
                color="black",
                label="rps",
            )

            ax.set_ylabel("Requests / second")
            ax.yaxis.set_major_locator(MaxNLocator(nbins=8))

            h, l = ax.get_legend_handles_labels()

        else:
            ax.yaxis.set_major_formatter(PercentFormatter(xmax=100))

            ax2 = ax.twinx()

            ax2.plot(
                list(x),
                rps,
                marker="x",
                linestyle="--",
                color="black",
                label="rps",
            )

            ax2.set_ylabel("Requests per second")

            h1, l1 = ax.get_legend_handles_labels()
            h2, l2 = ax2.get_legend_handles_labels()

            h, l = h1 + h2, l1 + l2

        ax.set_xticks(list(x))
        ax.set_xticklabels(iterations, rotation=90, fontsize=8)
        ax.grid(axis="y", linestyle="--", alpha=0.3)

        labels = [
            "Local success",
            "Fwd success",
            "Agent reject",
            "Local reject",
            "Fwd reject",
            "rps",
        ]

        handles = dict(zip(l, h))

        ax.legend(
            [handles[label] for label in labels if label in handles],
            [label for label in labels if label in handles],
            loc="upper center",
            bbox_to_anchor=(0.5, -0.18),
            ncol=3,
            fontsize=8,
        )

    fig, (ax1, ax2) = plt.subplots(
        2,
        1,
        figsize=(16, 10),
        sharex=True,
    )

    draw_plot(ax1)

    ax1.set(
        ylabel="Requests",
        ylim=(0, 105),
        title=f"{node}: request outcome distribution",
    )

    draw_plot(ax2, scale_to_rps=True)

    ax2.set(
        xlabel="Iteration",
        ylabel="Requests per second",
        title=f"{node}: request outcome distribution scaled to rps",
    )

    plt.tight_layout()

    fig.savefig(output_pdf, dpi=300, bbox_inches="tight")
    plt.close(fig)

    print(f"Saved plot: {output_pdf}")
    print()


def process_experiment(exp, node):
    input_csv = exp / "k6" / "global" / "k6_results_processed.csv"

    output_pdf = exp / "k6" / "global" / f"{node}_outcomes_over_iterations.pdf"

    output_csv = exp / "k6" / "global" / f"{node}_outcomes_over_iterations.csv"

    if not input_csv.exists():
        raise FileNotFoundError(f"Missing input CSV: {input_csv}")

    process_csv(
        input_csv,
        output_pdf,
        node,
        output_csv,
    )


def main():
    parser = argparse.ArgumentParser(
        description=("Plot request outcomes and requests/sec for one DFaaS node.")
    )

    parser.add_argument(
        "experiments",
        type=Path,
        nargs="*",
        help="Experiment directories",
    )

    parser.add_argument("--input-csv", type=Path, help="Direct input CSV")
    parser.add_argument("--output-pdf", type=Path, help="Output PDF")

    parser.add_argument("--node", required=True, help="Node to plot")

    args = parser.parse_args()

    # Direct CSV mode.
    if args.input_csv:
        if args.output_pdf is None:
            parser.error("--input-csv requires --output-pdf")

        process_csv(args.input_csv, args.output_pdf, args.node)
        return

    # Experiment directory mode.
    if not args.experiments:
        parser.error("provide experiments or use --input-csv")

    for exp in args.experiments:
        process_experiment(exp, args.node)


if __name__ == "__main__":
    main()
