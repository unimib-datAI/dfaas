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
        df["source_file"] = str(f)
        dfs.append(df)

    return pd.concat(dfs, ignore_index=True)


def rps_plot(df, iter_duration):
    if iter_duration <= 0:
        raise ValueError("iter_duration must be positive")

    # Split phases
    df_all_local = df.query("phase == 'all_local'")
    df_rl_agent = df.query("phase == 'rl_agent'")

    # Aggregate globally (sum across all nodes)
    all_rps = df_all_local.groupby("iteration").size() / iter_duration
    rl_rps = df_rl_agent.groupby("iteration").size() / iter_duration

    fig, ax = plt.subplots(figsize=(10, 6))

    ax.plot(all_rps.index, all_rps.values, label="all_local")
    ax.plot(rl_rps.index, rl_rps.values, label="rl_agent")

    ax.set_ylabel("RPS")
    ax.set_xlabel("Iteration")
    ax.set_title(f"Global RPS per iteration (iteration = {iter_duration}s)")
    ax.legend()
    ax.grid(True)

    fig.tight_layout()
    return fig


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Generate a global RPS plot. "
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
        help="Iteration duration in seconds",
    )

    args = parser.parse_args()

    input_path = Path(args.input)

    if not input_path.is_dir():
        raise NotADirectoryError(f"{input_path} is not a valid directory")

    df = load_data(input_path)

    fig = rps_plot(df, args.iter_duration)

    fig.savefig(args.output, format="pdf", bbox_inches="tight")
    print(f"Plot saved to: {args.output}")


if __name__ == "__main__":
    main()
