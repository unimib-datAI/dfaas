# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# This script searches for circular time offsets for five traces: node_a,
# node_b, node_c, node_f, node_g.
#
# Only the position of each trace in time is shifted. The optimizer selects
# offsets that reduce simultaneous peaks in the aggregate workload.
#
# INPUT: A JSON file containing a function entry "0" with the five nodes:
#
#   {
#     "0": {
#       "node_a": [r0, r0, r1, r1, r2, r2, ...],
#       "node_b": [r0, r0, r1, r1, r2, r2, ...],
#       "node_c": [r0, r0, r1, r1, r2, r2, ...],
#       "node_f": [r0, r0, r1, r1, r2, r2, ...],
#       "node_g": [r0, r0, r1, r1, r2, r2, ...]
#     }
#   }
#
# Each trace sample is duplicated. For example: [8, 8, 10, 10, 15, 15]
# represents the actual trace: [8, 10, 15]
#
# The script uses node_a as the reference trace and assumes all nodes contain
# the same trace pattern.
#
# OUTPUT: A JSON file named <input_name>_optimized.json. The output keeps the
# same structure of the input trace.
import json
import sys
from pathlib import Path

import numpy as np

# Nodes that receive the same input trace, only shifted in time.
NODES = ["node_a", "node_b", "node_c", "node_f", "node_g"]


def collapse_trace(trace):
    """
    The JSON stores every sample twice:
        [8, 8, 10, 10, 15, 15]

    Convert it back to the real trace:
        [8, 10, 15]
    """
    trace = np.asarray(trace, dtype=float)

    if len(trace) % 2:
        raise ValueError("Trace length must be even.")

    if not np.array_equal(trace[::2], trace[1::2]):
        raise ValueError("Trace samples are not duplicated.")

    return trace[::2]


def optimize(reference):
    """
    Find circular shifts for the five traces.

    node_a is kept at offset 0. The other nodes are moved in time
    to avoid having all request-rate peaks happen at the same moment.

    The optimization tries to:
      1. Minimize the highest total load peak.
      2. Minimize the variation of the total load.
      3. Minimize the p99 load as a final tie-breaker.
    """

    T = len(reference)

    # Precompute every possible circular shift of the trace.
    # shifts[i] contains the trace shifted by i samples.
    shifts = np.array([np.roll(reference, -i) for i in range(T)])

    def score(offsets):
        # Add the shifted traces of all nodes together.
        total = shifts[list(offsets)].sum(axis=0)

        # Lower peak and smoother workload are preferred.
        return (total.max(), total.std(), np.percentile(total, 99)), total

    rng = np.random.default_rng(42)

    # Initial guess: distribute the nodes evenly over the trace period.
    # This usually gives a good starting point for periodic workloads.
    best_offsets = [round(i * T / len(NODES)) % T for i in range(len(NODES))]
    best_offsets[0] = 0

    best_score, best_total = score(best_offsets)

    # Random global search:
    # Try many random combinations to escape bad local solutions.
    for _ in range(50000):
        offsets = [0, *rng.integers(0, T, len(NODES) - 1)]
        current_score, current_total = score(offsets)

        if current_score < best_score:
            best_score, best_offsets, best_total = current_score, offsets, current_total

    # Local refinement:
    # Improve one node at a time by checking every possible offset.
    # Repeat until no single-node change improves the result.
    improved = True

    while improved:
        improved = False

        for n in range(1, len(NODES)):
            for offset in range(T):
                if offset == best_offsets[n]:
                    continue

                candidate = best_offsets.copy()
                candidate[n] = offset

                candidate_score, candidate_total = score(candidate)

                if candidate_score < best_score:
                    best_score, best_offsets, best_total = (
                        candidate_score,
                        candidate,
                        candidate_total,
                    )
                    improved = True
                    break

    return tuple(best_offsets), best_total


def main():
    if len(sys.argv) != 2:
        print(f"Usage: {sys.argv[0]} <trace.json>")
        sys.exit(1)

    input_file = Path(sys.argv[1])

    with open(input_file) as f:
        data = json.load(f)

    nodes = data["0"]

    missing = [n for n in NODES if n not in nodes]
    if missing:
        raise ValueError(f"Missing nodes: {missing}")

    # Use node_a as the reference. All other nodes use the same trace.
    reference = collapse_trace(nodes["node_a"])

    offsets, total = optimize(reference)

    print(f"Input: {input_file}")

    for node, offset in zip(NODES, offsets):
        print(f"{node}: offset={offset}")

    print(
        f"max={total.max():.3f}, std={total.std():.3f}, p99={np.percentile(total, 99):.3f}"
    )

    # Apply the selected offsets and expand the samples again:
    # [8,10,15] -> [8,8,10,10,15,15]
    output = {
        "0": {
            node: np.repeat(np.roll(reference, -offset), 2).tolist()
            for node, offset in zip(NODES, offsets)
        }
    }

    output_file = input_file.parent / f"{input_file.stem}_optimized.json"

    with open(output_file, "w") as f:
        json.dump(output, f, indent=2)

    print(f"Written: {output_file}")


if __name__ == "__main__":
    main()
