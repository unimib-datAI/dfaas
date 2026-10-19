# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.

import argparse
from pathlib import Path

import matplotlib.pyplot as plt
import polars as pl
from matplotlib.ticker import FixedLocator

ITERATION_DURATION = 60
MAX_NODES = 5


def build_node_mapping(df):
    required = {"dfaas_node_id", "node"}
    missing = required - set(df.columns)
    if missing:
        raise ValueError(f"Missing columns: {sorted(missing)}")

    valid = df.filter(
        pl.col("dfaas_node_id").is_not_null()
        & (pl.col("dfaas_node_id") != "")
        & pl.col("node").is_not_null()
        & (pl.col("node") != "")
    )

    if valid.is_empty():
        raise ValueError("No valid dfaas_node_id values found.")

    check = valid.group_by("dfaas_node_id").agg(
        pl.col("node").n_unique().alias("names")
    )
    invalid = check.filter(pl.col("names") != 1)
    if invalid.height:
        raise ValueError(f"Invalid or ambiguous node mappings:\n{invalid}")

    return dict(valid.select(["dfaas_node_id", "node"]).unique().iter_rows())


def plot(
    nodes,
    forwards,
    failures,
    forwarded_failures,
    incoming,
    forwarded_incoming,
    output_pdf,
):
    iterations = sorted(incoming["iteration"].unique().to_list())
    if not iterations:
        raise ValueError("No iterations found in the input data.")

    x = list(range(len(iterations)))
    fig, axes = plt.subplots(
        len(nodes), 2, figsize=(16, 3.5 * len(nodes)), sharex=True, squeeze=False
    )

    def add_rps_line(ax, node):
        values = dict(
            incoming.filter(pl.col("node") == node)
            .select(["iteration", "rps"])
            .iter_rows()
        )
        line = [values.get(i, 0) for i in iterations]
        ax.plot(
            x, line, marker="x", linestyle="--", color="black", label="Incoming rate"
        )
        return line

    def add_forwarded_incoming_line(ax, node):
        forwarded = dict(
            forwarded_incoming.filter(pl.col("node") == node)
            .select(["iteration", "rps"])
            .iter_rows()
        )
        received = dict(
            incoming.filter(pl.col("node") == node)
            .select(["iteration", "rps"])
            .iter_rows()
        )
        line = [received.get(i, 0) + forwarded.get(i, 0) for i in iterations]
        ax.plot(
            x,
            line,
            marker="x",
            linestyle="-",
            color="green",
            label="Incoming + forwarded rate",
        )
        return line

    for row, node in enumerate(nodes):
        fax, eax = axes[row]
        data = forwards.filter(pl.col("node") == node)
        bottom = [0.0] * len(iterations)

        for recipient in sorted(data["forwarded_to"].unique().to_list()):
            values = dict(
                data.filter(pl.col("forwarded_to") == recipient)
                .select(["iteration", "rps"])
                .iter_rows()
            )
            bars = [values.get(i, 0) for i in iterations]
            if not any(v > 0 for v in bars):
                continue
            fax.bar(x, bars, bottom=bottom, label=recipient)
            bottom = [a + b for a, b in zip(bottom, bars)]

        fax.set_title(f"Forwarded requests on {node}")
        incoming_values = add_rps_line(fax, node)
        fax.set_ylim(
            0, max(max(incoming_values, default=0), max(bottom, default=0), 1) * 1.15
        )
        fax.set_ylabel("Requests/s")
        fax.grid(axis="y", linestyle="--", alpha=0.3)
        handles, labels = fax.get_legend_handles_labels()
        if handles:
            fax.legend(
                handles,
                labels,
                loc="upper center",
                bbox_to_anchor=(0.5, -0.18),
                ncol=4,
                fontsize=8,
            )

        local = dict(
            failures.filter(pl.col("node") == node)
            .select(["iteration", "rps"])
            .iter_rows()
        )
        local_bars = [local.get(i, 0) for i in iterations]
        forwarded = dict(
            forwarded_failures.filter(pl.col("node") == node)
            .select(["iteration", "rps"])
            .iter_rows()
        )
        forwarded_bars = [forwarded.get(i, 0) for i in iterations]

        eax.bar(x, local_bars, label="Local failure", color="red")
        eax.bar(
            x,
            forwarded_bars,
            bottom=local_bars,
            label="Forwarded failure",
            color="orange",
        )
        eax.set_title(f"Failures on {node}")
        incoming_values = add_rps_line(eax, node)
        forwarded_incoming_values = add_forwarded_incoming_line(eax, node)
        total_failures = [a + b for a, b in zip(local_bars, forwarded_bars)]
        ymax = max(
            max(total_failures, default=0),
            max(incoming_values, default=0),
            max(forwarded_incoming_values, default=0),
            1,
        )
        eax.set_ylim(0, ymax * 1.15)
        eax.set_ylabel("Requests/s")
        eax.grid(axis="y", linestyle="--", alpha=0.3)
        eax.legend(loc="upper center", bbox_to_anchor=(0.5, -0.18), ncol=4, fontsize=8)

    ticks = sorted(set(range(0, len(iterations), 5)) | {len(iterations) - 1})
    for ax in axes.flat:
        ax.xaxis.set_major_locator(FixedLocator(ticks))
        ax.set_xticks(ticks)
        ax.set_xticklabels([iterations[i] for i in ticks], rotation=90, fontsize=8)
        ax.tick_params(axis="x", labelbottom=True)

    axes[-1][0].set_xlabel("Iteration")
    axes[-1][1].set_xlabel("Iteration")
    fig.suptitle("DFaaS request forwarding and failures", fontsize=14)
    plt.tight_layout()
    fig.savefig(output_pdf, dpi=300, bbox_inches="tight")
    plt.close(fig)
    print(f"Output PDF: {output_pdf}")
    print()


def process_csv(input_csv, output_pdf):
    print(f"Input CSV: {input_csv}")
    df = pl.read_csv(input_csv)
    required = {
        "node",
        "iteration",
        "dfaas_node_id",
        "dfaas_forwarded_to",
        "http_status",
    }
    missing = required - set(df.columns)
    if missing:
        raise ValueError(f"CSV is missing required columns: {sorted(missing)}")
    if df.is_empty():
        raise ValueError("Input CSV contains no rows.")
    if df["node"].null_count():
        raise ValueError("CSV contains rows with a missing node name.")

    nodes = sorted(df["node"].unique().to_list())
    if not nodes:
        raise ValueError("CSV does not contain any nodes.")
    if len(nodes) > MAX_NODES:
        raise ValueError(
            f"Found {len(nodes)} nodes, but a maximum of {MAX_NODES} nodes is supported: {nodes}"
        )

    mapping = build_node_mapping(df)
    forwarded_ids = set(
        df.filter(pl.col("dfaas_forwarded_to").is_not_null())["dfaas_forwarded_to"]
        .unique()
        .to_list()
    )
    unmapped_ids = forwarded_ids - set(mapping)

    df = df.with_columns(
        pl.col("dfaas_forwarded_to").replace(mapping).alias("forwarded_to")
    )
    if unmapped_ids:
        df = df.with_columns(
            pl.when(pl.col("dfaas_forwarded_to").is_in(list(unmapped_ids)))
            .then(None)
            .otherwise(pl.col("forwarded_to"))
            .alias("forwarded_to")
        )

    incoming = (
        df.group_by(["node", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .sort(["node", "iteration"])
    )

    forwarded = pl.col("forwarded_to").is_not_null()
    forwards = (
        df.filter(forwarded)
        .group_by(["node", "iteration", "forwarded_to"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .sort(["node", "iteration", "forwarded_to"])
    )

    forwarded_incoming = (
        df.filter(forwarded)
        .group_by(["forwarded_to", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .rename({"forwarded_to": "node"})
        .sort(["node", "iteration"])
    )

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

    local_failure = (
        ~forwarded & (pl.col("http_status") != 200) & (pl.col("http_status") != 403)
    )
    failures = (
        df.filter(local_failure)
        .group_by(["node", "iteration"])
        .agg(pl.len().alias("requests"))
        .with_columns((pl.col("requests") / ITERATION_DURATION).alias("rps"))
        .sort(["node", "iteration"])
    )

    plot(
        nodes,
        forwards,
        failures,
        forwarded_failures,
        incoming,
        forwarded_incoming,
        output_pdf,
    )


def process_experiment(exp):
    input_csv = exp / "k6" / "global" / "k6_results_processed.csv"
    output_pdf = exp / "k6" / "global" / "forwarding_over_iterations.pdf"
    if not input_csv.exists():
        raise FileNotFoundError(f"Missing input CSV: {input_csv}")
    process_csv(input_csv, output_pdf)


def main():
    parser = argparse.ArgumentParser(
        description="Plot request forwarding and failures for all DFaaS nodes."
    )
    parser.add_argument(
        "experiments", type=Path, nargs="*", help="Experiment directories"
    )
    parser.add_argument("--input-csv", type=Path, help="Input CSV file")
    parser.add_argument("--output-pdf", type=Path, help="Output PDF file")
    args = parser.parse_args()

    if args.experiments:
        if args.input_csv or args.output_pdf:
            parser.error("--input-csv and --output-pdf cannot be used with experiments")
        for exp in args.experiments:
            process_experiment(exp.resolve())
        return

    if args.input_csv is None or args.output_pdf is None:
        parser.error("--input-csv and --output-pdf are required without experiments")
    process_csv(args.input_csv.resolve(), args.output_pdf.resolve())


if __name__ == "__main__":
    main()
