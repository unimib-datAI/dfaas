# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#

# Plot the requests per second (RPS) for each iteration of the input trace,
# considering only a single node (the first node in the trace by default).
#
# Each iteration lasts 60 seconds, so the x-axis shows only the iteration index.
#
# The generated PDF is saved as "rps_single_node_<node_name>.pdf" inside the
# given output directory.
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
        description="Plot the RPS per iteration of a single node of a JSON input trace."
    )
    parser.add_argument(
        "--input", required=True, type=Path, help="Path to input trace as JSON file."
    )
    parser.add_argument(
        "--output-dir",
        type=Path,
        default=Path("."),
        help="Directory where to save the PDF file (current directory by default).",
    )
    parser.add_argument(
        "--node",
        help="Name of the node to plot (first node in the trace by default).",
    )

    args = parser.parse_args()

    data = json.loads(args.input.read_text())

    # FIXME: Allow to have any function key!
    if "0" not in data:
        raise ValueError("Expected function key '0'.")
    nodes = data["0"]

    if not nodes:
        raise ValueError("No nodes found in the input trace.")

    node = args.node if args.node is not None else next(iter(nodes))
    if node not in nodes:
        raise ValueError(f"Node {node!r} not found, available nodes: {list(nodes)}")

    trace = np.asarray(nodes[node], dtype=float)
    iterations = np.arange(len(trace))

    fig, ax = plt.subplots(figsize=(12, 5))

    ax.plot(iterations, trace, linewidth=1.5)

    ax.set_xlabel("Iteration")
    ax.set_ylabel("Request rate (req/s)")
    ax.set_xlim(0, len(trace) - 1)
    ax.set_ylim(bottom=0)
    ax.grid(True, alpha=0.3)

    fig.tight_layout()

    args.output_dir.mkdir(parents=True, exist_ok=True)
    output = args.output_dir / f"rps_single_node_{node}.pdf"
    fig.savefig(output, format="pdf", bbox_inches="tight")
    print(f"Plot saved to: {output.resolve().as_posix()}")


if __name__ == "__main__":
    main()
