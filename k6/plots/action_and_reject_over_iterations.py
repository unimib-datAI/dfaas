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
MAX_NODES = 5


def plot(
    nodes,
    forwarding,
    failures,
    forwarded_failures,
    incoming,
    forwarded_incoming,
    output_pdf,
):
    """
    Generate a plot with one row per node and two columns:

      - left column: request handling decisions made by the node:
          * forwarded requests
          * requests processed locally
          * requests rejected by the agent
      - right column: local failures and failures of requests forwarded
        to the node by other nodes

    The incoming request rate is shown on both plots.
    """
    iterations = sorted(incoming["iteration"].unique().to_list())
    x = list(range(len(iterations)))

    fig, axes = plt.subplots(len(nodes), 2, figsize=(16, 3.5 * len(nodes)), sharex=True)

    if len(nodes) == 1:
        axes = [axes]

    def add_rps_line(ax, current_node):
        # Show incoming request rate for the current node.
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

    def add_forwarded_incoming_line(ax, current_node):
        # Show incoming rate + requests forwarded TO this node.
        values = forwarded_incoming.filter(pl.col("node") == current_node).select(
            ["iteration", "rps"]
        )

        forwarded_rps = dict(values.iter_rows())

        incoming_values = incoming.filter(pl.col("node") == current_node).select(
            ["iteration", "rps"]
        )
        incoming_rps = dict(incoming_values.iter_rows())

        line_values = [
            incoming_rps.get(iteration, 0) + forwarded_rps.get(iteration, 0)
            for iteration in iterations
        ]

        ax.plot(
            x,
            line_values,
            marker="x",
            linestyle="-",
            color="green",
            label="Incoming + forwarded rate",
        )

        return line_values

    for row, current_node in enumerate(nodes):
        decision_ax = axes[row][0]
        failure_ax = axes[row][1]

        # ------------------------------------------------------------------
        # Left column: request handling decisions
        # ------------------------------------------------------------------

        # The left plot shows a stacked bar for forwarded, rejected, and local
        # requests (in this order). Depending on the agent strategy, there may
        # be no forwarded, rejected or local requests.

        node_forwarding = forwarding.filter(
            (pl.col("node") == current_node) & (pl.col("decision") == "forwarded")
        )

        forwarded_values = dict(
            node_forwarding.group_by("iteration")
            .agg(pl.col("rps").sum().alias("rps"))
            .select(["iteration", "rps"])
            .iter_rows()
        )

        forwarded_bars = [
            forwarded_values.get(iteration, 0) for iteration in iterations
        ]

        rejected_data = (
            failures.filter(
                (pl.col("node") == current_node)
                & (pl.col("failure_type") == "agent_rejected")
            )
            if "failure_type" in failures.columns
            else None
        )

        if rejected_data is not None:
            rejected_values = dict(
                rejected_data.select(["iteration", "rps"]).iter_rows()
            )
            rejected_bars = [
                rejected_values.get(iteration, 0) for iteration in iterations
            ]
        else:
            rejected_bars = [0.0] * len(iterations)

        local_data = (
            forwarding.filter(
                (pl.col("node") == current_node) & (pl.col("decision") == "local")
            )
            if "decision" in forwarding.columns
            else None
        )

        if local_data is not None:
            local_values = dict(
                local_data.group_by("iteration")
                .agg(pl.col("rps").sum().alias("rps"))
                .select(["iteration", "rps"])
                .iter_rows()
            )
            local_bars = [local_values.get(iteration, 0) for iteration in iterations]
        else:
            local_bars = [0.0] * len(iterations)

        # Forwarded requests.
        decision_ax.bar(x, forwarded_bars, label="Forwarded")

        # Locally processed requests.
        decision_ax.bar(
            x,
            local_bars,
            bottom=forwarded_bars,
            label="Processed locally",
        )

        # The base point is required to show the rejected requests bars.
        local_plus_forwarded = [
            forwarded + local for forwarded, local in zip(forwarded_bars, local_bars)
        ]

        # Agent rejected requests.
        decision_ax.bar(
            x,
            rejected_bars,
            bottom=local_plus_forwarded,
            label="Agent rejected",
            color="red",
        )

        decision_ax.set_title(f"Request handling on {current_node}")

        incoming_values = add_rps_line(decision_ax, current_node)

        total_decision_values = [
            forwarded + local + rejected
            for forwarded, local, rejected in zip(
                forwarded_bars,
                local_bars,
                rejected_bars,
            )
        ]

        ymax = max(
            max(total_decision_values, default=0),
            max(incoming_values, default=0),
            1,
        )

        decision_ax.set_ylim(0, ymax * 1.15)
        decision_ax.set_ylabel("Requests/s")
        decision_ax.grid(axis="y", linestyle="--", alpha=0.3)

        decision_ax.legend(
            loc="upper center",
            bbox_to_anchor=(0.5, -0.18),
            ncol=4,
            fontsize=8,
        )

        # ------------------------------------------------------------------
        # Right column: local failures + forwarded failures
        # ------------------------------------------------------------------

        local_failure_data = failures.filter(
            (pl.col("node") == current_node)
            & (pl.col("failure_type") == "local_failure")
        )

        local_failure_values = dict(
            local_failure_data.select(["iteration", "rps"]).iter_rows()
        )

        local_failure_bars = [
            local_failure_values.get(iteration, 0) for iteration in iterations
        ]

        failure_ax.bar(
            x,
            local_failure_bars,
            label="Local failure",
            color="red",
        )

        forwarded_failure_data = forwarded_failures.filter(
            pl.col("node") == current_node
        )

        forwarded_failure_values = dict(
            forwarded_failure_data.select(["iteration", "rps"]).iter_rows()
        )

        forwarded_failure_bars = [
            forwarded_failure_values.get(iteration, 0) for iteration in iterations
        ]

        failure_ax.bar(
            x,
            forwarded_failure_bars,
            bottom=local_failure_bars,
            label="Forwarded failure",
            color="orange",
        )

        failure_ax.set_title(f"Failures on {current_node}")

        incoming_values = add_rps_line(failure_ax, current_node)
        forwarded_incoming_values = add_forwarded_incoming_line(
            failure_ax, current_node
        )

        total_failure_values = [
            local + forwarded
            for local, forwarded in zip(local_failure_bars, forwarded_failure_bars)
        ]

        ymax = max(
            max(total_failure_values, default=0),
            max(incoming_values, default=0),
            max(forwarded_incoming_values, default=0),
            1,
        )

        failure_ax.set_ylim(0, ymax * 1.15)
        failure_ax.set_ylabel("Requests/s")
        failure_ax.grid(axis="y", linestyle="--", alpha=0.3)

        failure_ax.legend(
            loc="upper center",
            bbox_to_anchor=(0.5, -0.18),
            ncol=4,
            fontsize=8,
        )

    # ----------------------------------------------------------------------
    # X-axis
    # ----------------------------------------------------------------------

    ticks = sorted(set(range(0, len(iterations), 5)) | {len(iterations) - 1})

    for row in range(len(nodes)):
        for col in range(2):
            ax = axes[row][col]

            ax.xaxis.set_major_locator(FixedLocator(ticks))

            ax.set_xticklabels(
                [iterations[i] for i in ticks],
                rotation=90,
                fontsize=8,
            )

            # Make sure tick labels are visible on every subplot.
            ax.tick_params(axis="x", labelbottom=True)

    axes[-1][0].set_xlabel("Iteration")
    axes[-1][1].set_xlabel("Iteration")

    fig.suptitle("DFaaS request handling and failures", fontsize=14)

    plt.tight_layout()

    fig.savefig(output_pdf, dpi=300, bbox_inches="tight")

    plt.close(fig)

    print(f"Saved plot: {output_pdf}")
    print()


def process_csv(input_csv, output_pdf):
    print(f"Loading: {input_csv}")

    df = pl.read_csv(input_csv)

    if "node" not in df.columns:
        raise ValueError("CSV does not contain 'node' column")

    nodes = sorted(df["node"].unique().to_list())

    if len(nodes) > MAX_NODES:
        raise ValueError(
            f"Found {len(nodes)} nodes, but a maximum of "
            f"{MAX_NODES} nodes is supported: {nodes}"
        )

    if not nodes:
        raise ValueError("CSV does not contain any nodes")

    # Create a mapping from DFaaS node IDs to node names.
    #
    # dfaas_forwarded_to stores IDs, not names, so this conversion must happen
    # before analysing forwarding behaviour.
    node_mapping = dict(
        df.filter(pl.col("dfaas_node_id").is_not_null())
        .select(["dfaas_node_id", "node"])
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

    # ----------------------------------------------------------------------
    # Incoming requests
    # ----------------------------------------------------------------------

    incoming = (
        df.group_by(["node", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
    )

    forwarded = pl.col("forwarded_to").is_not_null()

    # ----------------------------------------------------------------------
    # Request handling decisions
    # ----------------------------------------------------------------------

    # Forwarded requests.
    #
    # All forwarded requests are aggregated together, regardless of the
    # destination node.
    forwarded_decisions = (
        df.filter(forwarded)
        .group_by(["node", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .with_columns(pl.lit("forwarded").alias("decision"))
    )

    # Locally processed requests.
    #
    # A request is considered locally processed when:
    # - it was not forwarded
    # - the agent did not reject it with HTTP 403
    #
    # This includes both successful and failed local processing.
    local_processed = ~forwarded & (pl.col("http_status") != 403)

    local_decisions = (
        df.filter(local_processed)
        .group_by(["node", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .with_columns(pl.lit("local").alias("decision"))
    )

    # Agent-rejected requests.
    #
    # HTTP 403 identifies requests rejected by the agent.
    agent_rejected = pl.col("http_status") == 403

    rejected_decisions = (
        df.filter(agent_rejected)
        .group_by(["node", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .with_columns(pl.lit("rejected").alias("decision"))
    )

    forwarding = pl.concat(
        [forwarded_decisions, local_decisions, rejected_decisions],
        how="vertical",
    )

    # ----------------------------------------------------------------------
    # Forwarded incoming requests
    # ----------------------------------------------------------------------

    forwarded_incoming = (
        df.filter(forwarded)
        .group_by(["forwarded_to", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .rename({"forwarded_to": "node"})
        .sort(["node", "iteration"])
    )

    # ----------------------------------------------------------------------
    # Forwarded failures
    # ----------------------------------------------------------------------

    # Requests forwarded TO another node that subsequently failed there.
    #
    # These are requests that:
    # - were forwarded
    # - were forwarded to another node
    # - did not succeed (200)
    # - were not rejected by the agent (403)
    forwarded_failure = (
        forwarded
        & (pl.col("node") != pl.col("forwarded_to"))
        & (pl.col("http_status") != 200)
        & (pl.col("http_status") != 403)
    )

    forwarded_failures = (
        df.filter(forwarded_failure)
        .group_by(["forwarded_to", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .rename({"forwarded_to": "node"})
        .sort(["node", "iteration"])
    )

    # ----------------------------------------------------------------------
    # Local failures
    # ----------------------------------------------------------------------

    # These are requests that:
    # - were not forwarded
    # - were not rejected by the agent (403)
    # - did not succeed locally (200)
    local_failure = (
        ~forwarded & (pl.col("http_status") != 200) & (pl.col("http_status") != 403)
    )

    failures = (
        df.filter(local_failure)
        .group_by(["node", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .with_columns(pl.lit("local_failure").alias("failure_type"))
        .sort(["node", "iteration"])
    )

    # ----------------------------------------------------------------------
    # Agent rejections are needed by the left-hand plot.
    # ----------------------------------------------------------------------

    rejected_for_plot = (
        df.filter(agent_rejected)
        .group_by(["node", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .with_columns(pl.lit("agent_rejected").alias("failure_type"))
        .sort(["node", "iteration"])
    )

    failures = pl.concat(
        [failures, rejected_for_plot],
        how="vertical",
    )

    plot(
        nodes,
        forwarding,
        failures,
        forwarded_failures,
        incoming,
        forwarded_incoming,
        output_pdf,
    )


def process_experiment(exp):
    input_csv = exp / "k6" / "global" / "k6_results_processed.csv"
    output_pdf = exp / "k6" / "global" / "action_and_reject_over_iterations.pdf"

    if not input_csv.exists():
        raise FileNotFoundError(f"Missing input CSV: {input_csv}")

    process_csv(input_csv, output_pdf)


def main():
    parser = argparse.ArgumentParser(
        description=("Plot request handling and failures for all DFaaS nodes.")
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

    args = parser.parse_args()

    if args.experiments:
        if args.input_csv or args.output_pdf:
            parser.error("--input-csv and --output-pdf cannot be used with experiments")

        for exp in args.experiments:
            process_experiment(exp)

        return

    if args.input_csv is None or args.output_pdf is None:
        parser.error("--input-csv and --output-pdf are required without experiments")

    process_csv(args.input_csv, args.output_pdf)


if __name__ == "__main__":
    main()
