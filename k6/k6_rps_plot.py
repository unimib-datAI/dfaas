import argparse

import pandas as pd
import matplotlib.pyplot as plt


def rps_plot(df, iter_duration):
    df_all_local = df.query("phase == 'all_local'")
    df_rl_agent = df.query("phase == 'rl_agent'")

    if iter_duration <= 0:
        raise ValueError("iter_duration must be positive")

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


def main():
    parser = argparse.ArgumentParser(description="Generate RPS and duration plots.")
    parser.add_argument("--input", required=True, help="Path to input CSV file")
    parser.add_argument("--output", required=True, help="Path to output PDF file")
    parser.add_argument(
        "--iter-duration", type=int, default=60, help="Iteration duration in seconds"
    )

    args = parser.parse_args()

    df = pd.read_csv(args.input)

    fig = rps_plot(df, args.iter_duration)

    fig.savefig(args.output, format="pdf", bbox_inches="tight")
    print(f"Plot saved to: {args.output}")


if __name__ == "__main__":
    main()
