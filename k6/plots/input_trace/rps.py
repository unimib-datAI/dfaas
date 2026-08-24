# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#

# Plot the requests per second (RPS) for each iteration of the input trace. The
# generated figure contains two plots:
#
#   1. RPS for each node across iterations.
#   2. Total RPS across all nodes for each iteration, along with the average
#   RPS.
#
# The input trace must be a JSON file with the following structure:
#
# {
#     "0": {
#         "<node_name>": [<RPS per iteration>],
#         ...
#     }
# }
#
# The top-level key ("0") represents the function name, and the nested keys
# represent node names. Each value contains the RPS for that node at each
# iteration.
import argparse
import json
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np


def main():
    parser = argparse.ArgumentParser(
        description="Plot per-node RPS and total/average RPS across iterations of a JSON input trace."
    )
    parser.add_argument(
        "--input", required=True, type=Path, help="Path to input trace as JSON file."
    )
    parser.add_argument(
        "--output", required=True, type=Path, help="Path to output PDF file."
    )
    parser.add_argument(
        "--iter-duration",
        type=int,
        default=60,
        help="Iteration duration in seconds (60s by default).",
    )

    args = parser.parse_args()

    data = json.loads(args.input.read_text())

    # FIXME: Allow to have any function key!
    if "0" not in data:
        raise ValueError("Expected function key '0'.")
    nodes = data["0"]

    # Load traces exactly as they appear in the JSON.
    traces = {node: np.asarray(trace, dtype=float) for node, trace in nodes.items()}

    lengths = {len(trace) for trace in traces.values()}
    if len(lengths) != 1:
        raise ValueError(f"All node traces must have the same length: {lengths}")

    n_samples = next(iter(lengths))

    # Map all samples to the requested iteration duration.
    time = np.linspace(0, args.iter_duration, n_samples, endpoint=False)

    # Calculate the aggregated RPS for each iteration.
    total = np.sum(np.array(list(traces.values())), axis=0)

    fig, (ax1, ax2) = plt.subplots(
        2, 1, figsize=(12, 8), sharex=True, gridspec_kw={"height_ratios": [2, 1]}
    )

    # Individual node traces.
    for node, trace in traces.items():
        ax1.plot(time, trace, label=node, linewidth=1.5)

    ax1.set_ylabel("RPS")
    ax1.set_title("Input request traces")
    ax1.grid(True, alpha=0.3)
    ax1.legend()

    # Aggregated trace.
    ax2.plot(time, total, linewidth=1.8, label="Total")

    ax2.axhline(
        total.mean(), linestyle="--", linewidth=1, label=f"Mean ({total.mean():.2f})"
    )

    ax2.set_xlabel("Iteration")
    ax2.set_ylabel("RPS")
    ax2.set_title("Aggregate request rate")
    ax2.grid(True, alpha=0.3)
    ax2.legend()

    fig.tight_layout()

    fig.savefig(args.output, format="pdf", bbox_inches="tight")
    print(f"Plot saved to: {args.output.resolve().as_posix()}")


if __name__ == "__main__":
    main()
