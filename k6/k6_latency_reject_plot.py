import argparse

import pandas as pd
import matplotlib.pyplot as plt


def plot_latency_and_rejections(df, iter_duration=60):
    # -----------------------------
    # Latency per iteration
    # -----------------------------
    lat = df.groupby(["iteration", "phase"])["http_req_duration"].mean().reset_index()

    lat_all = lat[lat["phase"] == "all_local"]
    lat_rl = lat[lat["phase"] == "rl_agent"]

    # -----------------------------
    # Rejections per iteration
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
        color="red",
        label="all_local",
    )

    ax1.plot(
        lat_rl["iteration"], lat_rl["http_req_duration"], color="blue", label="rl_agent"
    )

    ax1.set_ylabel("Avg HTTP duration (ms)")
    ax1.set_title("Latency per iteration")
    ax1.legend()
    ax1.grid(True)

    # ---- Rejections ----
    ax2.plot(rej_all["iteration"], rej_all["rejected"], color="red", label="all_local")

    ax2.plot(rej_rl["iteration"], rej_rl["rejected"], color="blue", label="rl_agent")

    ax2.set_xlabel("Iteration")
    ax2.set_ylabel("Rejected requests")
    ax2.set_title("Rejections per iteration (HTTP != 200)")
    ax2.legend()
    ax2.grid(True)

    plt.tight_layout()

    return fig


def main():
    parser = argparse.ArgumentParser(
        description="Plot latency and rejects from processed k6 metrics. Make sure the input are CSV metrics processed with k6_parsed.py and the DFaaS agent strategy is RL agent"
    )

    parser.add_argument("--input", required=True, help="Path to input CSV")
    parser.add_argument("--output", required=True, help="Path to output PDF")
    parser.add_argument(
        "--iter-duration",
        type=int,
        default=60,
        help="Iteration duration in seconds (default: 60)",
    )

    args = parser.parse_args()

    df = pd.read_csv(args.input)

    fig = plot_latency_and_rejections(df, iter_duration=args.iter_duration)

    fig.savefig(args.output, format="pdf", bbox_inches="tight")
    print(f"Saved plot to {args.output}")


if __name__ == "__main__":
    main()
