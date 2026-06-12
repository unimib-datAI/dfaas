import argparse

import pandas as pd
import numpy as np
import matplotlib.pyplot as plt


def plot_latency_probability_density(df):
    successful_reqs = df[df["http_status"] == 200]

    # We calculate the bins here to guarantee the same bins in case of multiple
    # phases.
    bin_width_ms = 25
    max_latency = successful_reqs["http_req_duration"].max()
    bins = np.arange(0, max_latency + bin_width_ms, bin_width_ms)

    phases = []
    if "phase" in df.columns:
        # FIXME: Support more phases!
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

    # Plot data. We need density=True because the datasets may not be of the
    # same size.
    if len(phases) > 0:
        for phase in phases:
            data = phase["requests"]["http_req_duration"]

            # Use the same color both forst hist and stats for the same phase.
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
                np.percentile(data, 50), color=color, linestyle="dashed", linewidth=2
            )
            ax.axvline(
                np.percentile(data, 95), color=color, linestyle="dashdot", linewidth=2
            )
    else:
        data = successful_reqs["http_req_duration"]

        color = ax._get_lines.get_next_color()

        ax.hist(
            data,
            bins=bins,
            density=True,
            alpha=0.5,
            histtype="step",
            color=color,
            linewidth=2,
        )

        ax.axvline(data.mean(), color=color, linestyle="dotted", linewidth=2)
        ax.axvline(
            np.percentile(data, 50), color=color, linestyle="dashed", linewidth=2
        )
        ax.axvline(
            np.percentile(data, 95), color=color, linestyle="dashdot", linewidth=2
        )

    ax.set_xlabel("Latency (ms)")
    ax.set_ylabel("Probability density")
    ax.set_title(
        f"Latency Distribution (Probability Density) ({bin_width_ms}ms bin width)"
    )

    # Build two legends: one for the phases and one for the statistics.
    if len(phases) > 0:
        phase_legend = ax.legend(loc="upper right", title="Phases")
        ax.add_artist(phase_legend)

    # They don't have colors because they're valid for all phases!
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
        description="Plot the latency distribution of successfull requests from processed k6 metrics. Make sure the input are CSV metrics processed with k6_parser.py script. This script supports any strategy, because it automatically detects multiple phases checking the 'phase' column."
    )

    parser.add_argument("--input", required=True, help="Path to input CSV")
    parser.add_argument("--output", required=True, help="Path to output PDF")
    parser.add_argument("--iter-duration", help="Not used, kept for compatibility.")

    args = parser.parse_args()

    df = pd.read_csv(args.input)

    fig = plot_latency_probability_density(df)

    fig.savefig(args.output, format="pdf", bbox_inches="tight")
    print(f"Saved plot to {args.output}")


if __name__ == "__main__":
    main()
