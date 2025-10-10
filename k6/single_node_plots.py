#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

import argparse
from pathlib import Path
from datetime import datetime
from functools import cached_property

import pandas as pd
import matplotlib.pyplot as plt
from matplotlib.patches import Patch
import numpy as np


class Plots:
    """
    Generates and saves plots from DFaaS performance metrics.

    Each method computes a specific statistic, generates a corresponding plot,
    and saves it to disk. The data are taken from a DataFrame produced by
    reading k6 JSON results with the Pandas module. All plots are stored in a
    given directory.

    WARNING: Shared dataframes (e.g. self.df_http_reqs, self.df_timestamps) are
    reused across multiple methods for efficiency. If you need to modify them,
    make a copy first. The dataframes are cached and calculated only when called
    by the methods.

    Args:
        df (pandas.DataFrame): DataFrame with k6 metrics.
        output_dir (str or Path): Directory to save generated plots.

    Attributes:
        df (pandas.DataFrame): The provided metrics DataFrame.
        output_dir (Path): Directory for saving plots.
    """

    def __init__(self, df, output_dir):
        """
        Initialize Plots with the k6 metrics DataFrame and output directory.
        """
        self.df = df

        self.output_dir = Path(output_dir)
        self.output_dir.mkdir(parents=True, exist_ok=True)

        # Assume 'target' is always the same, this is the single node case!
        targets = self._df_dfaas_reqs["target"].unique()
        assert len(targets) == 1, f"Unsupported multiple targets: {targets}"
        # All requests are set to this DFaaS node.
        self.target = targets[0]

    @cached_property
    def df_http_reqs(self):
        """Filtered for HTTP requests of type 'Point' and metric "http_reqs"."""
        return self.df[
            (self.df["metric"] == "http_reqs") & (self.df["type"] == "Point")
        ]

    @cached_property
    def df_timestamps(self):
        """Extracted and binned timestamps for http requests."""
        df = pd.DataFrame(
            {"time": self.df_http_reqs["data"].apply(lambda x: x["time"])}
        )
        df["time"] = pd.to_datetime(df["time"])
        df["second"] = df["time"].dt.floor("s")
        return df

    @cached_property
    def _df_dfaas_reqs(self):
        """DFaaS requests metrics filtered and binned by seconds."""
        df_dfaas_reqs = self.df[
            (self.df["metric"] == "dfaas_requests") & (self.df["type"] == "Point")
        ]
        df = pd.DataFrame(
            {
                "time": df_dfaas_reqs["data"].apply(lambda x: x["time"]),
                "status": df_dfaas_reqs["data"].apply(lambda x: x["tags"]["status"]),
                "x_server": df_dfaas_reqs["data"].apply(
                    lambda x: x["tags"]["x_server"]
                ),
                "target": df_dfaas_reqs["data"].apply(lambda x: x["tags"]["target"]),
            }
        )
        df["time"] = pd.to_datetime(df["time"])
        df["second"] = df["time"].dt.floor("s")
        return df

    def _savefig(self, fig, name):
        """
        Save a matplotlib figure to the output directory as a PDF.

        Args:
            fig (matplotlib.figure.Figure): The figure object to save.
            name (str): The filename (without extension) for the figure.
        """
        output = self.output_dir / Path(f"{name}.pdf")
        fig.savefig(output)
        print(f"Saved figure: {output.as_posix()}")
        plt.close(fig)  # Prevent excessive resource usage.

    def responses_per_second(self):
        """
        Plot the number of HTTP responses per second.

        Saves the plot as 'responses_per_second.pdf' in the output directory.
        """
        # Get only seconds and count requests per second.
        df_reqs_per_sec = self.df_timestamps.groupby("second").count()

        seconds = df_reqs_per_sec.shape[0]  # Nr. of rows = total seconds.

        fig, ax = plt.subplots(figsize=(10, 5))
        ax.plot(range(seconds), df_reqs_per_sec.values)
        ax.set_xlabel("Time (seconds)")
        ax.set_ylabel("Responses per second")
        ax.grid(axis="both")
        ax.set_axisbelow(True)
        fig.tight_layout()

        self._savefig(fig, "responses_per_second")

    def responses_status_per_second(self):
        """
        Plot the number of responses per second, grouped by HTTP status code.

        The stackplot shows how many responses of each status occur per second.

        Saves the plot as 'responses_status_per_second.pdf' in the output
        directory.
        """
        # Extract timestamps and status codes. See responses_per_second()
        # method, it is similar.
        df_status = pd.DataFrame(
            {
                "time": self.df_http_reqs["data"].apply(lambda x: x["time"]),
                "status": self.df_http_reqs["data"].apply(
                    lambda x: x.get("tags", {}).get("status", "unknown")
                ),
            }
        )
        df_status["time"] = pd.to_datetime(df_status["time"])
        df_status["second"] = df_status["time"].dt.floor("s")

        # Group by second and status, count occurrences.
        pd_status_counts = (
            df_status.groupby(["second", "status"]).size().unstack(fill_value=0)
        )
        seconds = pd_status_counts.shape[0]

        # Prepare data for stackplot (one array for each status).
        stack_data = [
            pd_status_counts[status].values for status in pd_status_counts.columns
        ]

        fig, ax = plt.subplots(figsize=(10, 5))
        ax.stackplot(
            range(seconds),
            stack_data,
            labels=[status for status in pd_status_counts.columns],
        )
        ax.set_xlabel("Time (seconds)")
        ax.set_ylabel("Responses per second")
        ax.set_title("Responses per second by HTTP status")
        ax.grid(axis="both")
        ax.set_axisbelow(True)
        ax.legend(title="HTTP Status")
        fig.tight_layout()

        self._savefig(fig, "responses_status_per_second")

    def responses_host_per_second(self):
        """
        Plot the number of responses per second, grouped by DFaaS nodes.

        The stackplot shows how many responses were processed by each DFaaS node
        per second. The plot title includes the original DFaaS node destination.

        Saves the plot as 'responses_host_per_second.pdf' in the output
        directory.
        """
        # Group by second and x_server, count occurrences.
        pd_host_counts = (
            self._df_dfaas_reqs.groupby(["second", "x_server"])
            .size()
            .unstack(fill_value=0)
        )
        seconds = pd_host_counts.shape[0]

        # Prepare data for stackplot (one array for each x_server).
        stack_data = [pd_host_counts[host].values for host in pd_host_counts.columns]

        fig, ax = plt.subplots(figsize=(10, 5))
        ax.stackplot(
            range(seconds),
            stack_data,
            labels=[host for host in pd_host_counts.columns],
        )
        ax.set_xlabel("Time (seconds)")
        ax.set_ylabel("Requests per second")
        ax.set_title(f"Requests sent to {self.target}")
        ax.grid(axis="both")
        ax.set_axisbelow(True)
        ax.legend(title="DFaaS node IP")
        fig.tight_layout()

        self._savefig(fig, "responses_host_per_second")

    def responses_status_per_host(self):
        """
        Plot the number of responses per second for each DFaaS node, grouped by
        HTTP status code.

        Saves the plot as 'responses_status_per_host.pdf' in the output
        directory.
        """
        # The self._df_dfaas_reqs requests are already compacted by the second
        # column, extract also x_server and HTTP status code.
        df = pd.DataFrame(
            {
                "second": self._df_dfaas_reqs["second"],
                "x_server": self._df_dfaas_reqs["x_server"],
                "status": self._df_dfaas_reqs["status"],
            }
        )

        # Use relative seconds in the X axis, not the absolute times.
        abs_seconds = sorted(df["second"].unique())
        seconds = np.arange(len(abs_seconds))

        # Get all possibile hosts and assign a color.
        hosts = sorted(df["x_server"].unique())
        colors = plt.rcParams["axes.prop_cycle"].by_key()["color"]
        host_colors = {host: colors[i] for i, host in enumerate(hosts)}

        # Get all possible HTTP statuses and assign a pattern.
        statuses = sorted(df["status"].unique())
        dense_hatches = ["//", "\\\\", "||", "XX", "oo"]
        status_hatches = {status: dense_hatches[i] for i, status in enumerate(statuses)}

        # Manually build the legend with all hosts and statuses.
        legend_items = []
        for host in hosts:
            legend_items.append(Patch(facecolor=host_colors[host], label=host))
        for status in statuses:
            # By default facecolor is not "none", also the hatch color is taken
            # from the edgecolor.
            legend_items.append(
                Patch(
                    facecolor="none",
                    edgecolor="black",
                    hatch=status_hatches[status],
                    label=status,
                )
            )

        # Prepare stack data: for each host-status, counts per second.
        stack_data = []
        stack_colors = []
        stack_hatches = []
        for host in hosts:
            for status in statuses:
                counts = []
                for sec in abs_seconds:
                    count = df[
                        (df["second"] == sec)
                        & (df["x_server"] == host)
                        & (df["status"] == status)
                    ].shape[0]
                    counts.append(count)
                stack_data.append(counts)
                stack_colors.append(host_colors[host])
                stack_hatches.append(status_hatches[status])

        fig, ax = plt.subplots(figsize=(12, 6))

        # Calculate stack heights.
        stack_data = np.array(stack_data)  # shape: (num_stacks, num_seconds)
        cum_data = np.cumsum(stack_data, axis=0)

        # Draw stackplot manually with fill_between for hatches.
        prev = np.zeros_like(seconds)
        for i in range(stack_data.shape[0]):
            color = stack_colors[i]
            hatch = stack_hatches[i]
            top = cum_data[i]

            # Draw the line and the fill. Make the lines and colors a bit
            # lighter, to avoid killing the eyes.
            ax.fill_between(
                seconds,
                prev,
                top,
                facecolor=color,
                hatch=hatch,
                edgecolor="black",
                linewidth=0.5,
                alpha=0.7,
                step="pre",
            )
            prev = top

        ax.set_xlabel("Time (seconds since first response)")
        ax.set_ylabel("Responses per second")
        ax.set_title(f"Responses of requests sent to {self.target} by status and host")
        ax.grid(axis="both")
        ax.set_axisbelow(True)
        ax.legend(handles=legend_items, title="Host & HTTP Status")
        fig.tight_layout()

        self._savefig(fig, "responses_status_per_host")

    def responses_host_cumulative(self):
        """
        Plot the cumulative number of requests processed by each DFaaS node over
        time.

        The stackplot shows how the total number of requests processed by each
        DFaaS node grows over time. The plot title includes the original DFaaS
        node destination.

        Saves the plot as 'responses_host_cumulative.pdf' in the output
        directory.
        """
        # Group by second and x_server, count occurrences.
        pd_host_counts = (
            self._df_dfaas_reqs.groupby(["second", "x_server"])
            .size()
            .unstack(fill_value=0)
        )
        pd_host_counts_cum = pd_host_counts.cumsum()
        seconds = pd_host_counts_cum.shape[0]

        # Prepare data for stackplot (one array for each x_server).
        stack_data = [
            pd_host_counts_cum[host].values for host in pd_host_counts_cum.columns
        ]

        fig, ax = plt.subplots(figsize=(10, 5))
        ax.stackplot(
            range(seconds),
            stack_data,
            labels=[host for host in pd_host_counts_cum.columns],
        )
        ax.set_xlabel("Time (seconds)")
        ax.set_ylabel("Cumulative requests")
        ax.set_title(f"Cumulative requests sent to {self.target}")
        ax.grid(axis="both")
        ax.set_axisbelow(True)
        ax.legend(title="DFaaS node IP")
        fig.tight_layout()

        self._savefig(fig, "responses_host_cumulative")

    def responses_cumulative(self):
        """
        Plot the cumulative count of HTTP responses over time.

        Similar to responses_per_second, but values are summed up to show total
        responses so far.

        Saves the plot as 'responses_cumulative.pdf' in the output directory.
        """
        # Count requests per second.
        df_reqs_per_sec = self.df_timestamps.groupby("second").count()
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

        self._savefig(fig, "responses_cumulative")

    def responses_status_cumulative(self):
        """
        Plot the cumulative count of responses grouped by HTTP status code.

        Shows how the total number of responses of each HTTP status grows over
        time.

        Saves the plot as 'responses_status_cumulative.pdf' in the output
        directory.
        """
        df_status = pd.DataFrame(
            {
                "time": self.df_http_reqs["data"].apply(lambda x: x["time"]),
                "status": self.df_http_reqs["data"].apply(
                    lambda x: x.get("tags", {}).get("status", "unknown")
                ),
            }
        )
        df_status["time"] = pd.to_datetime(df_status["time"])
        df_status["second"] = df_status["time"].dt.floor("s")

        # Group by second and status, count occurrences.
        pd_status_counts = (
            df_status.groupby(["second", "status"]).size().unstack(fill_value=0)
        )
        pd_status_counts_cum = pd_status_counts.cumsum()
        seconds = pd_status_counts_cum.shape[0]

        # Prepare data for stackplot (one array for each status).
        stack_data = [
            pd_status_counts_cum[status].values
            for status in pd_status_counts_cum.columns
        ]

        fig, ax = plt.subplots(figsize=(10, 5))
        ax.stackplot(
            range(seconds),
            stack_data,
            labels=[status for status in pd_status_counts_cum.columns],
        )
        ax.set_xlabel("Time (seconds)")
        ax.set_ylabel("Cumulative responses")
        ax.set_title("Cumulative responses over time by HTTP status")
        ax.grid(axis="both")
        ax.set_axisbelow(True)
        ax.legend(title="HTTP Status")
        fig.tight_layout()

        self._savefig(fig, "responses_status_cumulative")

    def response_duration(self):
        """
        Plot the duration of HTTP responses for requests with status 200.

        Saves the plot as 'responses_duration.pdf' in the output directory.
        """
        # Filter for http_req_duration metrics of type Point.
        df_duration = self.df[
            (self.df["metric"] == "http_req_duration") & (self.df["type"] == "Point")
        ]

        # Extract response time, status code, and timestamp.
        df_plot = pd.DataFrame(
            {
                "duration": df_duration["data"].apply(lambda x: x["value"]),
                "status": df_duration["data"].apply(
                    lambda x: x.get("tags", {}).get("status", "unknown")
                ),
                "time": df_duration["data"].apply(lambda x: x["time"]),
            }
        )

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

        self._savefig(fig, "responses_duration")


def main():
    """
    Entry point for the plot generator script.
    """
    parser = argparse.ArgumentParser(description="Generate plots")
    parser.add_argument("result", type=Path, help="Path to the .json.gz file")
    parser.add_argument(
        "--output-dir",
        "-o",
        type=Path,
        default="plots",
        help="Output directory for plots (default: 'plots')",
    )

    args = parser.parse_args()

    try:
        df = pd.read_json(args.result, lines=True, compression="gzip")
    except Exception as e:
        print(f"Failed to read or parse {args.result.as_posix()}: {e}")
        exit(1)

    print("WARNING: This script is tailored for the single node test!")

    plots = Plots(df, args.output_dir)

    plots.responses_per_second()
    plots.responses_status_per_second()

    plots.responses_cumulative()
    plots.responses_status_cumulative()

    plots.response_duration()

    plots.responses_host_per_second()
    plots.responses_host_cumulative()
    plots.responses_status_per_host()


if __name__ == "__main__":
    main()
