import argparse

import pandas as pd
import matplotlib.pyplot as plt


def compute_latency_and_rejections(df):
    """
    Compute latency and rejection metrics per iteration for a single run.
    """
    # Latency per iteration
    lat = df.groupby("iteration")["http_req_duration"].mean().reset_index()

    # Rejections per iteration
    df = df.copy()
    df["rejected"] = df["http_status"] != 200

    rej = df.groupby("iteration")["rejected"].sum().reset_index()

    return lat, rej


def plot_latency_and_rejections_multi(dfs, iter_duration=60):
    """
    Plot average and standard deviation over multiple experiment runs.
    """

    latency_runs = []
    rejection_runs = []

    for df in dfs:
        lat, rej = compute_latency_and_rejections(df)

        latency_runs.append(lat.set_index("iteration")["http_req_duration"])
        rejection_runs.append(rej.set_index("iteration")["rejected"])

    # Align runs on iterations
    latency_df = pd.concat(latency_runs, axis=1)
    rejection_df = pd.concat(rejection_runs, axis=1)

    # Compute statistics
    latency_mean = latency_df.mean(axis=1)
    latency_std = latency_df.std(axis=1)

    rejection_mean = rejection_df.mean(axis=1)
    rejection_std = rejection_df.std(axis=1)

    iterations = latency_mean.index

    # -----------------------------
    # Plot
    # -----------------------------
    fig = plt.figure(figsize=(10, 8))

    ax1 = fig.add_subplot(2, 1, 1)
    ax2 = fig.add_subplot(2, 1, 2, sharex=ax1)

    # ---- Latency ----
    ax1.plot(
        iterations,
        latency_mean,
        color="blue",
        label="Average latency",
    )

    ax1.fill_between(
        iterations,
        (latency_mean - latency_std).clip(lower=0),  # Cannot be lower than 0.
        latency_mean + latency_std,
        color="blue",
        alpha=0.2,
        label="Std deviation",
    )

    ax1.set_ylabel("Avg HTTP duration (ms)")
    ax1.set_title("Latency per iteration (average ± std)")
    ax1.legend()
    ax1.grid(True)

    # ---- Rejections ----
    ax2.plot(
        iterations,
        rejection_mean,
        color="blue",
        label="Average rejections",
    )

    ax2.fill_between(
        iterations,
        (rejection_mean - rejection_std).clip(lower=0),  # Same as for ax1.
        rejection_mean + rejection_std,
        color="blue",
        alpha=0.2,
        label="Std deviation",
    )

    ax2.set_xlabel("Iteration")
    ax2.set_ylabel("Rejected requests")
    ax2.set_title("Rejections per iteration (HTTP != 200, average ± std)")
    ax2.legend()
    ax2.grid(True)

    plt.tight_layout()

    return fig


def main():
    description = (
        "Plot latency and rejected requests from multiple processed k6 metrics "
        "CSV files. Each input CSV represents one experiment run. The output "
        "plot shows average and standard deviation across runs."
    )

    parser = argparse.ArgumentParser(description=description)

    parser.add_argument(
        "--input",
        required=True,
        nargs="+",
        help="Paths to input CSV files (one per experiment run)",
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

    dfs = [pd.read_csv(path) for path in args.input]

    fig = plot_latency_and_rejections_multi(
        dfs,
        iter_duration=args.iter_duration,
    )

    fig.savefig(args.output, format="pdf", bbox_inches="tight")
    print(f"Saved plot to {args.output}")


if __name__ == "__main__":
    main()
