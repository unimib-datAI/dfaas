import argparse
from pathlib import Path

import pandas as pd
import matplotlib.pyplot as plt
from matplotlib import rcParams
from matplotlib.ticker import PercentFormatter, MultipleLocator
import matplotlib.lines as mlines


def compute_run_success_rate(df):
    """
    Convert raw request logs into per-(phase, iteration) success rate.
    This ensures each CSV is treated as one independent run.
    """
    df = df.copy()

    # Convert "http_req_failed" float column to "served" boolean.
    df["served"] = df["http_req_failed"] == 0
    df.drop("http_req_failed", axis=1, inplace=True)

    # We add a dummy phase if there are no phases found, this won't be
    # displayed.
    if "phase" not in df.columns:
        df["phase"] = "NOT_USED"

    # Per-run aggregation.
    run_stats = (
        df.groupby(["phase", "iteration"])["served"]
        .mean()
        .reset_index(name="success_rate")
    )

    return run_stats


def plot_single_run(df):
    # Convert "http_req_failed" float column to "served" boolean.
    df["served"] = df["http_req_failed"] == 0
    df.drop("http_req_failed", axis=1, inplace=True)

    # We add a dummy phase if there are no phases found, this won't be
    # displayed.
    if "phase" not in df.columns:
        df["phase"] = "NOT_USED"

    # Number (and labels) of iterations.
    iterations = sorted(df["iteration"].dropna().unique())

    phases = df["phase"].dropna().unique()

    # Calculate success rate per iteration and overall.
    success_rate_iter = df.groupby(["phase", "iteration"])["served"].mean()
    success_rate_total = df.groupby(["phase"])["served"].mean()

    fig, ax = plt.subplots(figsize=(10, 6))

    # Initialize Y-axis bounds, with +-inf any real data will update them.
    y_min, y_max = float("inf"), float("-inf")

    # Plot each phase data with a dedicated color.
    colors = iter(rcParams["axes.prop_cycle"].by_key()["color"])
    for phase in phases:
        color = next(colors)

        y = success_rate_iter[phase].reindex(iterations)

        # Update global Y-axis bounds.
        y_min = min(y_min, y.min())
        y_max = max(y_max, y.max())

        # If there are NaN or holes, just plot an hole for a specific iteration!
        ax.plot(iterations, y, color=color, label=phase)

        ax.axhline(
            y=success_rate_total[phase], color=color, linestyle="dashed", linewidth=2
        )

    ax.set_title("Request served rate per iteration")

    # Labels on X and Y axis.
    ax.set_xlabel("Iteration")
    ax.set_ylabel("Served requests")

    ax.set_xlim(0, 100)

    # Without margin, lines may sit exactly on the plot border.
    margin = 0.05
    ax.set_ylim(y_min - margin, y_max + margin)

    # Values are in the range [0, 1].
    ax.yaxis.set_major_formatter(PercentFormatter(1.0))

    # For Y axis: 0%, 10%, ..., 100%. For X axis: 0, 10, 20...
    ax.yaxis.set_major_locator(MultipleLocator(0.1))
    ax.xaxis.set_major_locator(MultipleLocator(10))

    # Show legend. Make sure to show at least the "Overall average" label!
    overall_label = mlines.Line2D([], [], color="black", linestyle="dashed")
    if len(phases) > 1:
        handles, labels = ax.get_legend_handles_labels()
        handles.append(overall_label)
        labels.append("Overall average")
        ax.legend(handles, labels)
    else:
        ax.legend(handles=[overall_label])

    ax.set_axisbelow(True)
    ax.grid(visible=True, which="both", alpha=0.3)

    fig.tight_layout()

    return fig


def plot_multiple_runs(df):
    iterations = sorted(df["iteration"].dropna().unique())
    phases = df["phase"].dropna().unique()

    fig, ax = plt.subplots(figsize=(10, 6))

    colors = iter(rcParams["axes.prop_cycle"].by_key()["color"])

    y_min, y_max = float("inf"), float("-inf")

    for phase in phases:
        color = next(colors)

        phase_df = df[df["phase"] == phase].set_index("iteration").reindex(iterations)

        mean = phase_df["mean"]
        min_v = phase_df["min"]
        max_v = phase_df["max"]

        # Update bounds using envelope.
        y_min = min(y_min, min_v.min())
        y_max = max(y_max, max_v.max())

        # Mean line.
        ax.plot(iterations, mean, color=color, label=f"{phase} (mean)")

        # Min/max envelope.
        ax.fill_between(
            iterations,
            min_v,
            max_v,
            color=color,
            alpha=0.2,
            linewidth=0,
        )

    ax.set_title("Request served rate per iteration (multiple runs)")
    ax.set_xlabel("Iteration")
    ax.set_ylabel("Served requests")

    ax.set_xlim(0, max(iterations) if iterations else 1)

    margin = 0.05
    ax.set_ylim(y_min - margin, y_max + margin)

    ax.yaxis.set_major_formatter(PercentFormatter(1.0))
    ax.yaxis.set_major_locator(MultipleLocator(0.1))
    ax.xaxis.set_major_locator(MultipleLocator(10))

    # Legend (mean lines only + envelope explanation).
    handles, labels = ax.get_legend_handles_labels()
    envelope = mlines.Line2D([], [], color="gray", alpha=0.2, linewidth=8)
    handles.append(envelope)
    labels.append("Min–Max range across runs")

    ax.legend(handles, labels)

    ax.set_axisbelow(True)
    ax.grid(visible=True, which="both", alpha=0.3)

    fig.tight_layout()
    return fig


def main():
    desc = (
        "Generates a plot of served HTTP request success rate over iterations. "
        "Supports multiple CSV files and aggregates results across runs."
    )

    parser = argparse.ArgumentParser(description=desc)

    parser.add_argument(
        "--input",
        required=True,
        type=Path,
        nargs="+",
        help="One or more input CSV files",
    )
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--iter-duration", help="Ignored (compatibility)")

    args = parser.parse_args()

    if len(args.input) == 1:
        df = pd.read_csv(args.input[0])

        fig = plot_single_run(df)
    else:
        all_runs = []

        # We merge all CSV into a single DataFrame.
        for csv_path in args.input:
            df = pd.read_csv(csv_path)

            run_stats = compute_run_success_rate(df)
            run_stats["run_id"] = csv_path.stem

            all_runs.append(run_stats)

        combined = pd.concat(all_runs, ignore_index=True)

        aggregated = (
            combined.groupby(["phase", "iteration"])["success_rate"]
            .agg(["mean", "std", "min", "max"])
            .reset_index()
        )

        fig = plot_multiple_runs(aggregated)

    fig.savefig(args.output, bbox_inches="tight")
    print(f"Saved plot to {args.output.resolve().absolute().as_posix()}")


if __name__ == "__main__":
    main()
