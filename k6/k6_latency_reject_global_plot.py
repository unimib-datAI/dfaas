import argparse
import pandas as pd
import matplotlib.pyplot as plt
from pathlib import Path


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
        df["source_file"] = str(f)  # optional
        dfs.append(df)

    return pd.concat(dfs, ignore_index=True)


def plot_latency_and_rejections(df, iter_duration=60):
    # -----------------------------
    # Latency per iteration (global avg)
    # -----------------------------
    lat = df.groupby(["iteration", "phase"])["http_req_duration"].mean().reset_index()

    lat_all = lat[lat["phase"] == "all_local"]
    lat_rl = lat[lat["phase"] == "rl_agent"]

    # -----------------------------
    # Rejections per iteration (global sum)
    # -----------------------------
    df = df.copy()
    df["rejected"] = df["http_status"] != 200

    rej = df.groupby(["iteration", "phase"])["rejected"].sum().reset_index()

    rej_all = rej[rej["phase"] == "all_local"]
    rej_rl = rej[rej["phase"] == "rl_agent"]

    # -----------------------------
    # Plot
    # -----------------------------
    fig = plt.figure(figsize=(10, 8))

    ax1 = fig.add_subplot(2, 1, 1)
    ax2 = fig.add_subplot(2, 1, 2, sharex=ax1)

    # ---- Latency ----
    ax1.plot(
        lat_all["iteration"],
        lat_all["http_req_duration"],
        label="all_local",
    )

    ax1.plot(
        lat_rl["iteration"],
        lat_rl["http_req_duration"],
        label="rl_agent",
    )

    ax1.set_ylabel("Avg HTTP duration (ms)")
    ax1.set_title("Global latency per iteration")
    ax1.legend()
    ax1.grid(True)

    # ---- Rejections ----
    ax2.plot(
        rej_all["iteration"],
        rej_all["rejected"],
        label="all_local",
    )

    ax2.plot(
        rej_rl["iteration"],
        rej_rl["rejected"],
        label="rl_agent",
    )

    ax2.set_xlabel("Iteration")
    ax2.set_ylabel("Rejected requests")
    ax2.set_title("Global rejections per iteration (HTTP != 200)")
    ax2.legend()
    ax2.grid(True)

    plt.tight_layout()
    return fig


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Generate global latency and rejection plots. "
            "Given an input directory, it reads all files matching "
            "'node_*/k6_results_processed.csv' and aggregates them."
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

    fig = plot_latency_and_rejections(df, iter_duration=args.iter_duration)

    fig.savefig(args.output, format="pdf", bbox_inches="tight")
    print(f"Saved plot to {args.output}")


if __name__ == "__main__":
    main()
