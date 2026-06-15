import argparse
from pathlib import Path

import pandas as pd
import numpy as np
import matplotlib.pyplot as plt


def load_data(input_dir):
    """Returns a merged DataFrame containing data from all
    'node_*/k6_results_processed.csv' files found within the specified directory
    path."""
    csv_files = list(input_dir.glob("node_*/k6_results_processed.csv"))

    if not csv_files:
        raise FileNotFoundError(
            f"No files found matching node_*/k6_results_processed.csv in {input_dir}"
        )

    dfs = []
    for file in csv_files:
        df = pd.read_csv(file)
        df["node_name"] = file.parent.name
        dfs.append(df)

    return pd.concat(dfs, ignore_index=True)


def plot_latency_probability_density(df):
    successful_reqs = df[df["http_status"] == 200]

    # Global bins.
    bin_width_ms = 25
    max_latency = successful_reqs["http_req_duration"].max()
    bins = np.arange(0, max_latency + bin_width_ms, bin_width_ms)

    # FIXME: Support arbitrary phases!
    phases = []
    if "phase" in df.columns:
        phases.append(
            {
                "label": "RL Agent",
                "requests": successful_reqs[successful_reqs["phase"] == "rl_agent"],
            }
        )
        phases.append(
            {
                "label": "All Local",
                "requests": successful_reqs[successful_reqs["phase"] == "all_local"],
            }
        )

    fig, ax = plt.subplots(figsize=(14, 6))

    ax.set_axisbelow(True)
    ax.grid(True, which="both", alpha=0.3)

    # Plot per phase
    if len(phases) > 0:
        for phase in phases:
            data = phase["requests"]["http_req_duration"]

            if len(data) == 0:
                continue

            color = ax._get_lines.get_next_color()

            ax.hist(
                data,
                bins=bins,
                density=True,
                alpha=0.5,
                histtype="step",
                linewidth=2,
                color=color,
                label=phase["label"],
            )

            ax.axvline(data.mean(), linestyle="dotted", color=color, linewidth=2)
            ax.axvline(
                np.percentile(data, 50),
                color=color,
                linestyle="dashed",
                linewidth=2,
            )
            ax.axvline(
                np.percentile(data, 95),
                color=color,
                linestyle="dashdot",
                linewidth=2,
            )
    # No phases found: single distribution.
    else:
        data = successful_reqs["http_req_duration"]

        color = ax._get_lines.get_next_color()

        ax.hist(
            data,
            bins=bins,
            density=True,
            alpha=0.5,
            histtype="step",
            linewidth=2,
            color=color,
        )

        ax.axvline(data.mean(), linestyle="dotted", color=color, linewidth=2)
        ax.axvline(
            np.percentile(data, 50),
            color=color,
            linestyle="dashed",
            linewidth=2,
        )
        ax.axvline(
            np.percentile(data, 95),
            color=color,
            linestyle="dashdot",
            linewidth=2,
        )

    ax.set_xlabel("Latency (ms)")
    ax.set_ylabel("Probability density")
    ax.set_title(
        f"Global Latency Distribution (Probability Density) ({bin_width_ms}ms bin width)"
    )

    if len(phases) > 0:
        phase_legend = ax.legend(loc="upper right", title="Phases")
        ax.add_artist(phase_legend)

    stat_handles = [
        plt.Line2D([0], [0], linestyle="dotted", color="black", label="Mean"),
        plt.Line2D([0], [0], linestyle="dashed", color="black", label="p50"),
        plt.Line2D([0], [0], linestyle="dashdot", color="black", label="p95"),
    ]

    ax.legend(handles=stat_handles, loc="center right", title="Statistics")

    fig.tight_layout()

    return fig


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Global latency distribution plot from distributed k6 nodes. "
            "Reads all node_*/k6_results_processed.csv files and aggregates them."
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
        help="Path to output PDF",
    )

    parser.add_argument(
        "--iter-duration",
        help="Not used, kept for compatibility.",
    )

    args = parser.parse_args()

    input_path = Path(args.input)

    if not input_path.is_dir():
        raise NotADirectoryError(f"{input_path} is not a valid directory")

    df = load_data(input_path)
    nodes = df["node_name"].unique().tolist()
    print(f"Found the following {len(nodes)} nodes under {input_path}: {nodes}")

    fig = plot_latency_probability_density(df)

    fig.savefig(args.output, format="pdf", bbox_inches="tight")
    print(f"Saved plot to {args.output}")


if __name__ == "__main__":
    main()
