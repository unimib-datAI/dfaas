import argparse
from pathlib import Path

import pandas as pd
import matplotlib.pyplot as plt
from matplotlib.ticker import PercentFormatter, MultipleLocator


def compute_run_success_rate(df):
    """
    Convert raw request logs into per-iteration success rate.
    Each CSV represents one independent experiment run.
    """
    df = df.copy()

    # Convert "http_req_failed" float column to "served" boolean.
    df["served"] = df["http_req_failed"] == 0

    # Compute success rate per iteration.
    run_stats = (
        df.groupby("iteration")["served"].mean().reset_index(name="success_rate")
    )

    return run_stats


def plot_multiple_runs(df):
    iterations = sorted(df["iteration"].dropna().unique())

    fig, ax = plt.subplots(figsize=(10, 6))

    mean = df.set_index("iteration")["mean"].reindex(iterations)
    std = df.set_index("iteration")["std"].reindex(iterations)

    # Clamp standard deviation envelope to valid served-rate range.
    lower = (mean - std).clip(lower=0)
    upper = (mean + std).clip(upper=1)

    ax.plot(
        iterations,
        mean,
        color="blue",
        label="Mean served rate",
    )

    ax.fill_between(
        iterations,
        lower,
        upper,
        color="blue",
        alpha=0.2,
        linewidth=0,
        label="Std deviation",
    )

    ax.set_title("Request served rate per iteration (multiple runs)")
    ax.set_xlabel("Iteration")
    ax.set_ylabel("Served requests")

    ax.set_xlim(0, max(iterations) if iterations else 1)
    ax.set_ylim(0, 1)

    ax.yaxis.set_major_formatter(PercentFormatter(1.0))
    ax.yaxis.set_major_locator(MultipleLocator(0.1))
    ax.xaxis.set_major_locator(MultipleLocator(10))

    ax.legend()

    ax.set_axisbelow(True)
    ax.grid(visible=True, which="both", alpha=0.3)

    fig.tight_layout()

    return fig


def main():
    desc = (
        "Generates a plot of served HTTP request success rate over iterations "
        "aggregated across multiple experiment runs."
    )

    parser = argparse.ArgumentParser(description=desc)

    parser.add_argument(
        "--input",
        required=True,
        type=Path,
        nargs="+",
        help="Input CSV files (one per experiment run)",
    )

    parser.add_argument(
        "--output",
        required=True,
        type=Path,
        help="Output PDF path",
    )

    parser.add_argument(
        "--iter-duration",
        help="Ignored (compatibility)",
    )

    args = parser.parse_args()

    all_runs = []

    for csv_path in args.input:
        df = pd.read_csv(csv_path)

        run_stats = compute_run_success_rate(df)
        run_stats["run_id"] = csv_path.stem

        all_runs.append(run_stats)

    combined = pd.concat(all_runs, ignore_index=True)

    aggregated = (
        combined.groupby("iteration")["success_rate"].agg(["mean", "std"]).reset_index()
    )

    fig = plot_multiple_runs(aggregated)

    fig.savefig(args.output, bbox_inches="tight")

    print(f"Saved plot to {args.output.resolve().absolute().as_posix()}")


if __name__ == "__main__":
    main()
