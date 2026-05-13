#!/usr/bin/env python
import argparse

import pandas as pd
import matplotlib.pyplot as plt


def cpu_usage_plot(df):
    # Ensure timestamps are parsed correctly
    df = df.copy()
    df["timestamp"] = pd.to_datetime(df["timestamp"])

    # Sort by timestamp to preserve execution order
    df = df.sort_values("timestamp").reset_index(drop=True)

    # Create iteration index from 0..n
    df["iteration"] = range(len(df))

    # Extract instance label
    # Example: instance=dfaas-node-c -> dfaas-node-c
    instance_label = df["labels"].iloc[0].split("instance=")[-1]

    # Extract container name from metric name
    # Example: mlimage_cpu_usage_percent_1m -> mlimage
    metric_name = df["metric"].iloc[0]
    container_name = metric_name.split("_cpu_usage_percent")[0]

    # Input values are already normalized between 0 and 100.
    cpu_usage = df["value"]

    fig, ax = plt.subplots(figsize=(10, 6))

    ax.plot(
        df["iteration"],
        cpu_usage,
        color="blue",
        label=instance_label,
    )

    ax.set_xlabel("Iteration")
    ax.set_ylabel("CPU Usage (%)")

    ax.set_title(
        f"CPU Usage over Iterations - "
        f"Container: {container_name} - "
        f"Instance: {instance_label}"
    )

    ax.set_ylim(-5, 105)

    ax.legend()
    ax.grid(True)

    fig.tight_layout()

    return fig


def main():
    parser = argparse.ArgumentParser(
        description="Generate CPU usage plot from CSV metrics."
    )

    parser.add_argument(
        "--input",
        required=True,
        help="Path to input CSV file",
    )

    parser.add_argument(
        "--output",
        required=True,
        help="Path to output PDF file",
    )

    args = parser.parse_args()

    # CSV uses ';' as separator
    df = pd.read_csv(args.input, sep=";")

    # Ensure numeric conversion
    df["value"] = pd.to_numeric(df["value"])

    fig = cpu_usage_plot(df)

    fig.savefig(args.output, format="pdf", bbox_inches="tight")

    print(f"Plot saved to: {args.output}")


if __name__ == "__main__":
    main()
