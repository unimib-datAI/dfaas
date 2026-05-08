import argparse
from pathlib import Path

import pandas as pd
import matplotlib.pyplot as plt


def load_data(input_dir):
    """Load and concatenate all node_*/k6_results_processed.csv files."""

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


def plot_status_stackplots(df, iter_duration=60):
    """
    Create two global stack plots (one per phase) showing the number
    of requests per iteration grouped by HTTP status code.
    """

    # -----------------------------
    # Aggregate counts globally by:
    # iteration / phase / status
    # -----------------------------
    status_counts = (
        df.groupby(["iteration", "phase", "http_status"])
        .size()
        .reset_index(name="count")
    )

    # Normalize status code formatting
    status_counts["http_status"] = (
        status_counts["http_status"].fillna(-1).astype(int).astype(str)
    )

    # Fixed phase ordering
    phases = ["all_local", "rl_agent"]

    # Keep status ordering consistent across plots
    all_status_codes = sorted(status_counts["http_status"].unique())

    # -----------------------------
    # Plot
    # -----------------------------
    fig, axes = plt.subplots(
        2,
        1,
        figsize=(12, 8),
        sharex=True,
        constrained_layout=True,
    )

    for ax, phase in zip(axes, phases):
        phase_df = status_counts[status_counts["phase"] == phase]

        # Pivot:
        # rows    -> iteration
        # columns -> status code
        # values  -> request count
        pivot = (
            phase_df.pivot_table(
                index="iteration",
                columns="http_status",
                values="count",
                fill_value=0,
            )
            .reindex(columns=all_status_codes, fill_value=0)
            .sort_index()
        )

        iterations = pivot.index.values

        stack_values = [pivot[status].values for status in all_status_codes]

        ax.stackplot(
            iterations,
            stack_values,
            labels=all_status_codes,
            alpha=0.85,
        )

        ax.set_title(f"Global requests per iteration by HTTP status ({phase})")

        ax.set_ylabel("Requests")
        ax.grid(True)

        ax.legend(
            title="HTTP Status",
            loc="upper left",
            bbox_to_anchor=(1.01, 1.0),
        )

    axes[-1].set_xlabel("Iteration")

    return fig


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Generate global HTTP status stack plots. "
            "Given an input directory, it reads all files matching "
            "'node_*/k6_results_processed.csv' and aggregates them. "
            "Warning: the CSV files must be from RL agent strategy, "
            "since the script assumes the existence of 'phase' column."
        )
    )

    parser.add_argument(
        "--input",
        required=True,
        help="Path to input directory containing node_* subdirectories",
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
        help="Iteration duration in seconds (default: 60)",
    )

    args = parser.parse_args()

    input_path = Path(args.input)

    if not input_path.is_dir():
        raise NotADirectoryError(f"{input_path} is not a valid directory")

    df = load_data(input_path)

    fig = plot_status_stackplots(
        df,
        iter_duration=args.iter_duration,
    )

    fig.savefig(args.output, format="pdf", bbox_inches="tight")

    print(f"Saved plot to {args.output}")


if __name__ == "__main__":
    main()
