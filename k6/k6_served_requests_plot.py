import argparse
from pathlib import Path

import pandas as pd
import matplotlib.pyplot as plt
from matplotlib import rcParams
from matplotlib.ticker import PercentFormatter
from matplotlib.ticker import MultipleLocator
import matplotlib.lines as mlines


def plot(df):
    # Convert "http_req_failed" float column to "served" boolean.
    df["served"] = df["http_req_failed"] == 0
    df.drop("http_req_failed", axis=1, inplace=True)

    # Number (and labels) of iterations.
    iterations = sorted(df["iteration"].dropna().unique())

    # We add a dummy phase if there are no phases found, this won't be
    # displayed.
    if "phase" not in df.columns:
        df["phase"] = "NOT_USED"
    phases = df["phase"].dropna().unique()

    # Calculate success rate per iteration and overall.
    success_rate_iter = df.groupby(["phase", "iteration"])["served"].mean()
    success_rate_total = df.groupby(["phase"])["served"].mean()

    fig, ax = plt.subplots(figsize=(10, 6))

    # Plot each phase data with a dedicated color.
    colors = iter(rcParams["axes.prop_cycle"].by_key()["color"])
    for phase in phases:
        color = next(colors)

        # If there are NaN or holes, just plot an hole for a specific iteration!
        ax.plot(
            iterations,
            success_rate_iter[phase].reindex(iterations),
            color=color,
            label=phase,
        )

        ax.axhline(
            y=success_rate_total[phase], color=color, linestyle="dashed", linewidth=2
        )

    ax.set_title("Request served rate per iteration")

    # Labels on X and Y axis.
    ax.set_xlabel("Iteration")
    ax.set_ylabel("Served requests")

    ax.set_xlim(0, 100)
    ax.set_ylim(0, 1)

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


def main():
    desc = (
        "This script generates a plot showing the success rate of served "
        "HTTP requests over time. It computes whether each request was "
        "successfully served (http_req_failed == 0) and aggregates results "
        "per iteration and per phase. If no phase is found, all data is "
        "treated as a single default phase."
    )
    parser = argparse.ArgumentParser(description=desc)

    parser.add_argument("--input", required=True, type=Path, help="Path to input CSV")
    parser.add_argument("--output", required=True, type=Path, help="Path to output PDF")

    args = parser.parse_args()

    df = pd.read_csv(args.input)

    fig = plot(df)
    fig.savefig(args.output, bbox_inches="tight")

    print(f"Saved plot to {args.output.as_posix()}")


if __name__ == "__main__":
    main()
