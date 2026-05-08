import argparse
from pathlib import Path

import pandas as pd
import matplotlib.pyplot as plt


def load_data(input_dir):
    """
    Load and concatenate all node_*/k6_results_processed.csv files.
    """

    csv_files = list(input_dir.glob("node_*/k6_results_processed.csv"))

    if not csv_files:
        raise FileNotFoundError(
            f"No files found matching node_*/k6_results_processed.csv in {input_dir}"
        )

    dfs = []

    for f in csv_files:
        df = pd.read_csv(f)
        df["source_file"] = str(f)  # optional/debug
        dfs.append(df)

    return pd.concat(dfs, ignore_index=True)


def classify_request(row):
    """
    Classify requests for forwarding behavior plot.

    Rules:
    - Forwarded:
        dfaas_forwarded_to is present
    - Rejected directly:
        dfaas_forwarded_to is empty AND http_status == 560
    - Local:
        dfaas_forwarded_to is empty
    """

    forwarded = pd.notna(row["dfaas_forwarded_to"]) and (
        str(row["dfaas_forwarded_to"]).strip() != ""
    )

    if forwarded:
        return "Forwarded"

    if int(row["http_status"]) == 560:
        return "Rejected directly"

    return "Local"


def shorten_node_id(node_id):
    """
    Keep only the last 5 chars of destination node id.
    """

    if pd.isna(node_id):
        return "unknown"

    node_id = str(node_id)

    return node_id[-5:]


def plot_global_forwarding_behavior(df, iter_duration=60):
    """
    Generate two GLOBAL stackplots:

    1. Global RL-agent request handling behavior:
       - Local
       - Forwarded
       - Rejected directly

    2. Global RL-agent forwarded requests
       grouped by destination node.
    """

    # ---------------------------------------------------------
    # Filter RL-agent phase only
    # ---------------------------------------------------------

    rl_df = df[df["phase"] == "rl_agent"].copy()

    # =========================================================
    # FIGURE 1
    # Request classification
    # =========================================================

    rl_df["request_type"] = rl_df.apply(
        classify_request,
        axis=1,
    )

    class_counts = (
        rl_df.groupby(["iteration", "request_type"]).size().reset_index(name="count")
    )

    request_order = [
        "Local",
        "Forwarded",
        "Rejected directly",
    ]

    class_pivot = (
        class_counts.pivot_table(
            index="iteration",
            columns="request_type",
            values="count",
            fill_value=0,
        )
        .reindex(columns=request_order, fill_value=0)
        .sort_index()
    )

    # =========================================================
    # FIGURE 2
    # Forwarded destination distribution
    # =========================================================

    forwarded_df = rl_df[rl_df["dfaas_forwarded_to"].notna()].copy()

    forwarded_df = forwarded_df[
        forwarded_df["dfaas_forwarded_to"].astype(str).str.strip() != ""
    ]

    forwarded_df["dest_short"] = forwarded_df["dfaas_forwarded_to"].apply(
        shorten_node_id
    )

    dest_counts = (
        forwarded_df.groupby(["iteration", "dest_short"])
        .size()
        .reset_index(name="count")
    )

    all_destinations = sorted(dest_counts["dest_short"].unique())

    dest_pivot = (
        dest_counts.pivot_table(
            index="iteration",
            columns="dest_short",
            values="count",
            fill_value=0,
        )
        .reindex(columns=all_destinations, fill_value=0)
        .sort_index()
    )

    # =========================================================
    # Plot
    # =========================================================

    fig, axes = plt.subplots(
        2,
        1,
        figsize=(14, 10),
        sharex=True,
        constrained_layout=True,
    )

    # ---------------------------------------------------------
    # FIGURE 1
    # Global request behavior
    # ---------------------------------------------------------

    ax1 = axes[0]

    class_colors = {
        "Local": "tab:blue",
        "Forwarded": "tab:green",
        "Rejected directly": "tab:red",
    }

    class_iterations = class_pivot.index.values

    class_stack_values = [class_pivot[col].values for col in request_order]

    class_stack_colors = [class_colors[col] for col in request_order]

    ax1.stackplot(
        class_iterations,
        class_stack_values,
        labels=request_order,
        colors=class_stack_colors,
        alpha=0.85,
    )

    ax1.set_title("Global RL-agent requests per iteration by handling type")

    ax1.set_ylabel("Requests")

    ax1.grid(True)

    ax1.legend(
        title="Handling type",
        loc="upper left",
        bbox_to_anchor=(1.01, 1.0),
    )

    # ---------------------------------------------------------
    # FIGURE 2
    # Global forwarding destinations
    # ---------------------------------------------------------

    ax2 = axes[1]

    dest_iterations = dest_pivot.index.values

    dest_stack_values = [dest_pivot[col].values for col in all_destinations]

    ax2.stackplot(
        dest_iterations,
        dest_stack_values,
        labels=all_destinations,
        alpha=0.85,
    )

    ax2.set_title("Global RL-agent forwarded requests per iteration by destination")

    ax2.set_xlabel("Iteration")

    ax2.set_ylabel("Forwarded requests")

    ax2.grid(True)

    ax2.legend(
        title="Destination ID (last 5 chars)",
        loc="upper left",
        bbox_to_anchor=(1.01, 1.0),
    )

    return fig


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Generate global RL-agent forwarding plots. "
            "Given an input directory, the script reads all "
            "files matching "
            "'node_*/k6_results_processed.csv' "
            "and aggregates them."
        )
    )

    parser.add_argument(
        "--input",
        required=True,
        help=("Path to input directory containing node_* subdirectories"),
    )

    parser.add_argument(
        "--output",
        required=True,
        help="Path to output PDF file",
    )

    parser.add_argument(
        "--iter-duration",
        type=int,
        default=60,
        help=("Iteration duration in seconds (default: 60)"),
    )

    args = parser.parse_args()

    input_path = Path(args.input)

    if not input_path.is_dir():
        raise NotADirectoryError(f"{input_path} is not a valid directory")

    df = load_data(input_path)

    fig = plot_global_forwarding_behavior(
        df,
        iter_duration=args.iter_duration,
    )

    fig.savefig(
        args.output,
        format="pdf",
        bbox_inches="tight",
    )

    print(f"Saved plot to {args.output}")


if __name__ == "__main__":
    main()
