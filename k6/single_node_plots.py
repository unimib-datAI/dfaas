#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

# TODO: Add plots for forwarding (see X-Server header from dfaas_requests
# metric).

import argparse
from pathlib import Path
from datetime import datetime

import pandas as pd
import matplotlib.pyplot as plt

def plot_responses_per_second(df):
    df_http_reqs = df[(df["metric"] == "http_reqs") & (df["type"] == "Point")]

    # Extract only "time" and convert as real timestamp, not string.
    df_timestamps = pd.DataFrame({"time": df_http_reqs["data"].apply(lambda x: x["time"])})
    df_timestamps["time"] = pd.to_datetime(df_timestamps["time"])
    # Bin the timestamps by the closest second.
    df_timestamps["second"] = df_timestamps["time"].dt.floor('s')

    # Get only seconds and count requests per second.
    df_reqs_per_sec = df_timestamps.groupby("second").count()

    seconds = df_reqs_per_sec.shape[0]  # Nr. of rows = total seconds.

    fig, ax = plt.subplots(figsize=(10, 5))
    ax.plot(range(seconds), df_reqs_per_sec.values)
    ax.set_xlabel("Time (seconds)")
    ax.set_ylabel("Responses per second")
    ax.grid(axis="both")
    ax.set_axisbelow(True)
    fig.tight_layout()

    base_dir = Path("plots")
    base_dir.mkdir(parents=True, exist_ok=True)
    output = base_dir / Path("responses_per_second.pdf")
    fig.savefig(output)
    print(f"Saved figure: {output.as_posix()}")

def plot_responses_status_per_second(df):
    """
    Plots the number of responses per second, grouped by HTTP status code.
    """
    df_http_reqs = df[(df["metric"] == "http_reqs") & (df["type"] == "Point")]

    # Extract timestamps and status codes. See plot_responses_per_second, it is
    # similar.
    df_status = pd.DataFrame({
        "time": df_http_reqs["data"].apply(lambda x: x["time"]),
        "status": df_http_reqs["data"].apply(lambda x: x.get("tags", {}).get("status", "unknown"))
    })
    df_status["time"] = pd.to_datetime(df_status["time"])
    df_status["second"] = df_status["time"].dt.floor('s')

    # Group by second and status, count occurrences.
    pd_status_counts = df_status.groupby(["second", "status"]).size().unstack(fill_value=0)
    seconds = pd_status_counts.shape[0]

    # Prepare data for stackplot (one array for each status).
    stack_data = [pd_status_counts[status].values for status in pd_status_counts.columns]

    fig, ax = plt.subplots(figsize=(10, 5))
    ax.stackplot(range(seconds), stack_data, labels=[status for status in pd_status_counts.columns])
    ax.set_xlabel("Time (seconds)")
    ax.set_ylabel("Responses per second")
    ax.set_title("Responses per second by HTTP status")
    ax.grid(axis="both")
    ax.set_axisbelow(True)
    ax.legend(title="HTTP Status")
    fig.tight_layout()

    base_dir = Path("plots")
    base_dir.mkdir(parents=True, exist_ok=True)
    output = base_dir / Path("responses_status_per_second.pdf")
    fig.savefig(output)
    print(f"Saved figure: {output.as_posix()}")

def plot_responses_cumulative(df):
    """
    Show response rate as cumulative (similar to plot_responses_per_second but cumulative).
    """
    df_http_reqs = df[(df["metric"] == "http_reqs") & (df["type"] == "Point")]

    # Extract only "time" and convert as real timestamp.
    df_timestamps = pd.DataFrame({"time": df_http_reqs["data"].apply(lambda x: x["time"])})
    df_timestamps["time"] = pd.to_datetime(df_timestamps["time"])
    df_timestamps["second"] = df_timestamps["time"].dt.floor('s')

    # Count requests per second.
    df_reqs_per_sec = df_timestamps.groupby("second").count()
    df_reqs_per_sec_cum = df_reqs_per_sec.cumsum()

    seconds = df_reqs_per_sec_cum.shape[0]

    fig, ax = plt.subplots(figsize=(10, 5))
    ax.plot(range(seconds), df_reqs_per_sec_cum.values)
    ax.set_xlabel("Time (seconds)")
    ax.set_ylabel("Cumulative responses")
    ax.set_title("Cumulative responses over time")
    ax.grid(axis="both")
    ax.set_axisbelow(True)
    fig.tight_layout()

    base_dir = Path("plots")
    base_dir.mkdir(parents=True, exist_ok=True)
    output = base_dir / Path("responses_cumulative.pdf")
    fig.savefig(output)
    print(f"Saved figure: {output.as_posix()}")

def plot_responses_status_cumulative(df):
    """
    Similar to plot_responses_status_per_second but cumulative.
    """
    df_http_reqs = df[(df["metric"] == "http_reqs") & (df["type"] == "Point")]

    df_status = pd.DataFrame({
        "time": df_http_reqs["data"].apply(lambda x: x["time"]),
        "status": df_http_reqs["data"].apply(lambda x: x.get("tags", {}).get("status", "unknown"))
    })
    df_status["time"] = pd.to_datetime(df_status["time"])
    df_status["second"] = df_status["time"].dt.floor('s')

    # Group by second and status, count occurrences.
    pd_status_counts = df_status.groupby(["second", "status"]).size().unstack(fill_value=0)
    pd_status_counts_cum = pd_status_counts.cumsum()
    seconds = pd_status_counts_cum.shape[0]

    # Prepare data for stackplot (one array for each status).
    stack_data = [pd_status_counts_cum[status].values for status in pd_status_counts_cum.columns]

    fig, ax = plt.subplots(figsize=(10, 5))
    ax.stackplot(range(seconds), stack_data, labels=[status for status in pd_status_counts_cum.columns])
    ax.set_xlabel("Time (seconds)")
    ax.set_ylabel("Cumulative responses")
    ax.set_title("Cumulative responses over time by HTTP status")
    ax.grid(axis="both")
    ax.set_axisbelow(True)
    ax.legend(title="HTTP Status")
    fig.tight_layout()

    base_dir = Path("plots")
    base_dir.mkdir(parents=True, exist_ok=True)
    output = base_dir / Path("responses_status_cumulative.pdf")
    fig.savefig(output)
    print(f"Saved figure: {output.as_posix()}")

def plot_response_duration(df):
    """
    Plots the response duration (http_req_duration) for requests with status 200.
    Uses seconds (from first request) as X axis.
    """
    # Filter for http_req_duration metrics of type Point.
    df_duration = df[(df["metric"] == "http_req_duration") & (df["type"] == "Point")]

    # Extract response time, status code, and timestamp.
    df_plot = pd.DataFrame({
        "duration": df_duration["data"].apply(lambda x: x["value"]),
        "status": df_duration["data"].apply(lambda x: x.get("tags", {}).get("status", "unknown")),
        "time": df_duration["data"].apply(lambda x: x["time"])
    })

    # Keep only responses with HTTP status 200.
    df_plot_200 = df_plot[df_plot["status"] == "200"].copy()

    # Convert "time" column to datetime.
    df_plot_200["time"] = pd.to_datetime(df_plot_200["time"])

    # Set the first response time as zero reference. All other responses will
    # have relative time to the first (instead of absolute).
    t0 = df_plot_200["time"].iloc[0]
    df_plot_200["seconds"] = (df_plot_200["time"] - t0).dt.total_seconds()

    # Plot durations over seconds since start.
    fig, ax = plt.subplots(figsize=(10, 5))
    ax.plot(df_plot_200["seconds"].values, df_plot_200["duration"].values)
    ax.set_xlabel("Time (seconds since first response)")
    ax.set_ylabel("Response duration (ms)")
    ax.grid(axis="both")
    ax.set_axisbelow(True)
    fig.tight_layout()

    base_dir = Path("plots")
    base_dir.mkdir(parents=True, exist_ok=True)
    output = base_dir / Path("response_duration.pdf")
    fig.savefig(output)
    print(f"Saved figure: {output.as_posix()}")

def main():
    parser = argparse.ArgumentParser(description="Generate plots")
    parser.add_argument("result", type=Path, help="Path to the .json.gz file")

    args = parser.parse_args()

    try:
        df = pd.read_json(args.result, lines=True, compression="gzip")
    except Exception as e:
        print(f"Failed to read or parse {args.result.as_posix()}: {e}")
        exit(1)

    print("WARNING: This script is tailored for the single node test!")

    # Get the test duration (from http_reqs metric, first and last req).
    df_http_reqs = df[(df["metric"] == "http_reqs") & (df["type"] == "Point")]
    df_first_req, df_last_req = df_http_reqs.iloc[0], df_http_reqs.iloc[-1]

    # Python datetime only supports microseconds, not nanoseconds (like k6
    # does), so we need to remove it.
    start_str = df_first_req.data["time"][:-1][:26]
    end_str = df_last_req.data["time"][:-1][:26]

    start = datetime.strptime(start_str, "%Y-%m-%dT%H:%M:%S.%f")
    end = datetime.strptime(end_str, "%Y-%m-%dT%H:%M:%S.%f")

    duration = end - start

    print("Duration:", duration)
    print("Total seconds:", duration.total_seconds())

    plot_responses_per_second(df)
    plot_responses_status_per_second(df)

    plot_responses_cumulative(df)
    plot_responses_status_cumulative(df)

    plot_response_duration(df)

if __name__ == "__main__":
    main()
