import argparse

import pandas as pd
import matplotlib.pyplot as plt


def rps_plot_rlstrategy(df, iter_duration):
    df_all_local = df.query("phase == 'all_local'")
    df_rl_agent = df.query("phase == 'rl_agent'")

    all_rps = df_all_local.groupby("iteration").size() / iter_duration
    rl_rps = df_rl_agent.groupby("iteration").size() / iter_duration

    fig, ax = plt.subplots(figsize=(10, 6))

    ax.plot(all_rps.index, all_rps.values, color="red", label="all_local")
    ax.plot(rl_rps.index, rl_rps.values, color="blue", label="rl_agent")
    ax.set_ylabel("RPS")
    ax.set_xlabel("Iteration")
    ax.set_title(
        f"Request Per Seconds (RPS) per iteration (with fixed iteration of {iter_duration}s)"
    )
    ax.legend()
    ax.grid(True)

    fig.tight_layout()

    return fig


def rps_plot(df, iter_duration):
    rps = df.groupby("iteration").size() / iter_duration

    fig, ax = plt.subplots(figsize=(10, 6))

    ax.plot(rps.index, rps.values, color="red")
    ax.set_ylabel("RPS")
    ax.set_xlabel("Iteration")
    ax.set_title(
        f"Request Per Seconds (RPS) per iteration (with fixed iteration of {iter_duration}s)"
    )
    ax.grid(True)

    fig.tight_layout()

    return fig


def main():
    parser = argparse.ArgumentParser(description="Generate RPS and duration plots.")
    parser.add_argument("--input", required=True, help="Path to input CSV file.")
    parser.add_argument("--output", required=True, help="Path to output PDF file.")
    parser.add_argument(
        "--iter-duration",
        type=int,
        default=60,
        help="Iteration duration in seconds (60s by default).",
    )

    args = parser.parse_args()

    if args.iter_duration <= 0:
        raise ValueError("iter_duration must be positive")

    df = pd.read_csv(args.input)

    # We need to support CSV files without the "phase" column, it means the
    # experiment has used only a single load balancing strategy.
    if "phase" not in df.columns:
        df["phase"] = "IGNORED"

    if df["phase"].nunique(dropna=True) > 1:
        fig = rps_plot_rlstrategy(df, args.iter_duration)
    else:
        fig = rps_plot(df, args.iter_duration)

    fig.savefig(args.output, format="pdf", bbox_inches="tight")
    print(f"Plot saved to: {args.output}")


if __name__ == "__main__":
    main()
