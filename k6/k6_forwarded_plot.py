import argparse

import pandas as pd
import matplotlib.pyplot as plt


def classify_request(row):
    """
    Classify requests for the first stackplot.

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
    """Keep only the last 5 chars of destination node id."""

    if pd.isna(node_id):
        return "unknown"

    node_id = str(node_id)

    return node_id[-5:]


def plot_forwarding_behavior(df, iter_duration=60):
    """
    Generate two stackplots:

    1. Request handling behavior per iteration:
       - Local
       - Forwarded
       - Rejected directly

    2. Forwarded requests destination distribution per iteration.
    """

    # -----------------------------
    # Filter only RL phase
    # -----------------------------
    rl_df = df[df["phase"] == "rl_agent"].copy()

    # =========================================================
    # FIGURE 1
    # Request classification stackplot
    # =========================================================

    rl_df["request_type"] = rl_df.apply(classify_request, axis=1)

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
    # Forwarded destination stackplot
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
    # PLOT
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

    ax1.set_title("RL agent requests per iteration by handling type")

    ax1.set_ylabel("Requests")

    ax1.grid(True)

    ax1.legend(
        title="Handling type",
        loc="upper left",
        bbox_to_anchor=(1.01, 1.0),
    )

    # ---------------------------------------------------------
    # FIGURE 2
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

    ax2.set_title("RL agent forwarded requests per iteration by destination")

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
            "Plot RL-agent forwarding behavior and forwarding "
            "destinations from processed k6 CSV metrics."
        )
    )

    parser.add_argument(
        "--input",
        required=True,
        help="Path to input CSV",
    )

    parser.add_argument(
        "--output",
        required=True,
        help="Path to output PDF",
    )

    parser.add_argument(
        "--iter-duration",
        type=int,
        default=60,
        help="Iteration duration in seconds (default: 60)",
    )

    args = parser.parse_args()

    df = pd.read_csv(args.input)

    fig = plot_forwarding_behavior(
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
