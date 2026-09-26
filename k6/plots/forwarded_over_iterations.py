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


def plot(node, nodes, forwards, failures, incoming, output_pdf):
    """
    Generate a plot showing:
      - incoming requests/sec for every node
      - forwarded requests for the selected node
      - local failures for the remaining nodes
    """
    iterations = sorted(incoming["iteration"].unique().to_list())
    x = list(range(len(iterations)))

    fig, axes = plt.subplots(
        len(nodes),
        1,
        figsize=(16, 3.5 * len(nodes)),
        sharex=True,
    )

    if len(nodes) == 1:
        axes = [axes]

    def add_rps_line(ax, current_node):
        # Show incoming request rate for every node.
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
            # For the selected node, show forwarded requests only.
            #
            # dfaas_forwarded_to contains the destination node ID, which was
            # already converted to the node name before this step.
            data = forwards.filter(pl.col("iteration").is_in(iterations))

            recipients = sorted(data["forwarded_to"].unique().to_list())

            bottom = [0.0] * len(iterations)

            for recipient in recipients:
                values = data.filter(pl.col("forwarded_to") == recipient).select(
                    [
                        "iteration",
                        "rps",
                    ]
                )

                values = dict(values.iter_rows())

                bar_values = [values.get(iteration, 0) for iteration in iterations]

                ax.bar(
                    x,
                    bar_values,
                    bottom=bottom,
                    label=recipient,
                )

                bottom = [a + b for a, b in zip(bottom, bar_values)]

            ax.set_title(f"Forwarded requests on {current_node}")

        else:
            # For all other nodes, show only local failures.
            data = failures.filter(pl.col("node") == current_node)

            values = dict(
                data.select(
                    [
                        "iteration",
                        "rps",
                    ]
                ).iter_rows()
            )

            bar_values = [values.get(iteration, 0) for iteration in iterations]

            ax.bar(
                x,
                bar_values,
                label="Local failure",
                color="red",
            )

            ax.set_title(f"Local failures on {current_node}")

        # Incoming rate is always shown for every node.
        incoming_values = add_rps_line(
            ax,
            current_node,
        )

        ymax = max(
            max(incoming_values, default=0),
            1,
        )

        ax.set_ylim(
            0,
            ymax * 1.15,
        )

        ax.set_ylabel("Requests/s")

        ax.grid(
            axis="y",
            linestyle="--",
            alpha=0.3,
        )

        ax.legend(
            loc="upper center",
            bbox_to_anchor=(0.5, -0.18),
            ncol=4,
            fontsize=8,
        )

    # Show tick on X-axis every 5 iterations.
    ticks = sorted(set(range(0, len(iterations), 5)) | {len(iterations) - 1})

    for ax in axes:
        ax.xaxis.set_major_locator(FixedLocator(ticks))

        ax.set_xticklabels(
            [iterations[i] for i in ticks],
            rotation=90,
            fontsize=8,
        )

        # Make sure tick labels are visible on every subplot.
        ax.tick_params(
            axis="x",
            labelbottom=True,
        )

    axes[-1].set_xlabel("Iteration")

    plt.tight_layout()

    fig.savefig(
        output_pdf,
        dpi=300,
        bbox_inches="tight",
    )

    plt.close(fig)

    print(f"Saved plot: {output_pdf}")
    print()


def process_csv(input_csv, output_pdf, node):
    print(f"Loading: {input_csv}")

    df = pl.read_csv(input_csv)

    if "node" not in df.columns:
        raise ValueError("CSV does not contain 'node' column")

    nodes = sorted(df["node"].unique().to_list())

    if node not in nodes:
        raise ValueError(f"Node {node!r} not found. Available: {nodes}")

    # Create a mapping from DFaaS node IDs to node names.
    #
    # dfaas_forwarded_to stores IDs, not names, so this conversion must happen
    # before analysing forwarding behaviour.
    node_mapping = dict(
        df.filter(pl.col("dfaas_node_id").is_not_null())
        .select(
            [
                "dfaas_node_id",
                "node",
            ]
        )
        .unique()
        .iter_rows()
    )

    # Empty forwarded_to means the request was not forwarded.
    df = df.with_columns(
        pl.when(pl.col("dfaas_forwarded_to") == "")
        .then(None)
        .otherwise(pl.col("dfaas_forwarded_to"))
        .alias("dfaas_forwarded_to")
    )

    # Replace forwarded destination IDs with node names.
    df = df.with_columns(
        pl.col("dfaas_forwarded_to").replace(node_mapping).alias("forwarded_to")
    )

    # Count incoming requests for every node.
    #
    # This is independent of forwarding/failure. Every request received by a
    # node contributes to its incoming rate.
    incoming = (
        df.group_by(
            [
                "node",
                "iteration",
            ]
        )
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
    )

    forwarded = pl.col("forwarded_to").is_not_null()

    # Forwarded requests generated by the selected node.
    forwards = (
        df.filter((pl.col("node") == node) & forwarded)
        .group_by(
            [
                "iteration",
                "forwarded_to",
            ]
        )
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .sort(
            [
                "iteration",
                "forwarded_to",
            ]
        )
    )

    # Local failures on all nodes except the selected node.
    #
    # These are requests that:
    # - were not forwarded
    # - were not rejected by the agent (403)
    # - did not succeed locally (200)
    local_failure = (
        ~forwarded & (pl.col("http_status") != 200) & (pl.col("http_status") != 403)
    )

    failures = (
        df.filter(local_failure & (pl.col("node") != node))
        .group_by(
            [
                "node",
                "iteration",
            ]
        )
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .sort(
            [
                "node",
                "iteration",
            ]
        )
    )

    plot(
        node,
        nodes,
        forwards,
        failures,
        incoming,
        output_pdf,
    )


def process_experiment(exp, node):
    input_csv = exp / "k6" / "global" / "k6_results_processed.csv"

    output_pdf = exp / "k6" / "global" / f"{node}_forwarding_over_iterations.pdf"

    if not input_csv.exists():
        raise FileNotFoundError(f"Missing input CSV: {input_csv}")

    process_csv(
        input_csv,
        output_pdf,
        node,
    )


def main():
    parser = argparse.ArgumentParser(
        description=("Plot request forwarding and local failures for DFaaS nodes.")
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
        help="Input CSV file",
    )

    parser.add_argument(
        "--output-pdf",
        type=Path,
        help="Output PDF file",
    )

    parser.add_argument(
        "--node",
        required=True,
        help="Node to analyse",
    )

    args = parser.parse_args()

    if args.experiments:
        if args.input_csv or args.output_pdf:
            parser.error("--input-csv and --output-pdf cannot be used with experiments")

        for exp in args.experiments:
            process_experiment(
                exp,
                args.node,
            )

        return

    if args.input_csv is None or args.output_pdf is None:
        parser.error("--input-csv and --output-pdf are required without experiments")

    process_csv(
        args.input_csv,
        args.output_pdf,
        args.node,
    )


if __name__ == "__main__":
    main()
