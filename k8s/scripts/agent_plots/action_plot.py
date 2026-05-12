import argparse
import pandas as pd
import matplotlib.pyplot as plt


def plot_action_distribution(df):
    # ---- rename node columns ----
    df = df.rename(
        columns={
            c: ("node_" + c[-4:]) if c.startswith("node_") else c for c in df.columns
        }
    )

    # ---- select action columns ----
    action_cols = [
        c for c in df.columns if c in ["local", "reject"] or c.startswith("node_")
    ]

    df[action_cols] = df[action_cols].astype(float)

    # ---- timestep ----
    df["t"] = range(len(df))

    # ---- plot ----
    fig, ax = plt.subplots(figsize=(12, 5))

    ax.stackplot(df["t"], [df[c] for c in action_cols], labels=action_cols, alpha=0.9)

    ax.legend(loc="upper right")
    ax.set_xlabel("Timestep")
    ax.set_ylabel("Probability distribution")
    ax.set_title("Action distribution per timestep")
    ax.grid(True)

    fig.tight_layout()

    return fig


def main():
    parser = argparse.ArgumentParser(
        description="Plot action distribution over time from CSV"
    )

    parser.add_argument("--input", required=True, help="Path to input CSV file")
    parser.add_argument(
        "--output",
        required=True,
        help="Path to output file (e.g. plot.pdf or plot.png)",
    )

    args = parser.parse_args()

    # ---- load data ----
    df = pd.read_csv(args.input)

    # ---- generate plot ----
    fig = plot_action_distribution(df)

    # ---- save ----
    fig.savefig(args.output, bbox_inches="tight")
    print(f"Plot saved to: {args.output}")


if __name__ == "__main__":
    main()
