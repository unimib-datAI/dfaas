import argparse

import pandas as pd
import matplotlib.pyplot as plt


def rps_plot(df, iter_duration):
    df_all_local = df.query("phase == 'all_local'")
    df_rl_agent = df.query("phase == 'rl_agent'")

    if iter_duration <= 0:
        raise ValueError("iter_duration must be positive")

    # RPS (fixed 60s)
    all_rps = df_all_local.groupby("iteration").size() / iter_duration
    rl_rps = df_rl_agent.groupby("iteration").size() / iter_duration

    # Detected duration
    all_duration = df_all_local.groupby("iteration")["timestamp"].agg(
        lambda x: x.max() - x.min()
    )
    rl_duration = df_rl_agent.groupby("iteration")["timestamp"].agg(
        lambda x: x.max() - x.min()
    )

    # Deviation from expected 60s
    all_dev = all_duration - 60
    rl_dev = rl_duration - 60

    # Create figure
    fig = plt.figure(figsize=(10, 8))

    ax1 = fig.add_subplot(2, 1, 1)
    ax2 = fig.add_subplot(2, 1, 2)

    # ---- RPS ----
    ax1.plot(all_rps.index, all_rps.values, color="red", label="all_local")
    ax1.plot(rl_rps.index, rl_rps.values, color="blue", label="rl_agent")
    ax1.set_ylabel("RPS")
    ax1.set_title(
        f"Request Per Seconds (RPS) per iteration (with fixed iteration duration of {iter_duration}s)"
    )
    ax1.legend()
    ax1.grid(True)

    # ---- Deviation ----
    ax2.plot(
        all_dev.index, all_dev.values, linestyle="--", color="red", label="all_local"
    )
    ax2.plot(
        rl_dev.index, rl_dev.values, linestyle="--", color="blue", label="rl_agent"
    )
    ax2.axhline(0, linestyle=":", linewidth=1)
    ax2.set_xlabel("Iteration")
    ax2.set_ylabel("Seconds")
    ax2.set_title(f"Detected iteration duration deviation from {iter_duration}s")
    ax2.legend()
    ax2.grid(True)

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

    # Load data
    df = pd.read_csv(args.input)

    # Generate plot
    fig = rps_plot(df, args.iter_duration)

    # Save to file (PDF or any extension supported by matplotlib)
    fig.savefig(args.output, format="pdf", bbox_inches="tight")
    print(f"Plot saved to: {args.output}")


if __name__ == "__main__":
    main()
