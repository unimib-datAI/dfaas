#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# Custom utility to generate input request traces for the experiment testbench.
#
# The testbench has five nodes: node_a, node_b, node_c, node_f, and node_g. The
# input JSON file contains a semi-realistic request-rate trace for node_a. The
# other nodes use a fixed rate of 5 requests per second.
#
# The script generates four configurations with 2, 3, 4, and 5 nodes that use
# the semi-realistic trace. For each configuration, the script shifts the same
# trace for the additional nodes. It selects the offsets to reduce simultaneous
# request-rate peaks in the total workload. Note that the script does not change
# the trace values.
#
# The generated JSON files can be used to compare different numbers of active
# nodes while keeping the same request pattern. The total number of requests
# will anyway be increased when more nodes use the semi-realistic trace.
#
# For example, if the input file is 'high_only_a_other_fixed_5.json', the script
# generates: 'high_only_a_other_fixed_5_optimized_2_special.json',
# 'high_only_a_other_fixed_5_optimized_3_special.json', ... to
# 'high_only_a_other_fixed_5_optimized_5_special.json'.
import json
import sys
from pathlib import Path

import numpy as np

# The special nodes use the same trace, only shifted. For N special nodes, the
# first N nodes in this list are used.
SPECIAL_NODES = [
    "node_a",
    "node_b",
    "node_c",
    "node_f",
    "node_g",
]

# Fixed constant rate for the remaining nodes.
FIXED_RATE = 5


def collapse_trace(trace):
    """
    The input contains every sample twice:

        [8, 8, 10, 10, 15, 15, ...]

    Return the actual trace:

        [8, 10, 15, ...]
    """
    trace = np.asarray(trace, dtype=float)

    if len(trace) % 2:
        raise ValueError("Trace length must be even.")

    a = trace[::2]
    b = trace[1::2]

    if not np.array_equal(a, b):
        raise ValueError("The trace does not contain identical duplicated samples.")

    return a


def optimize(reference, n_special):
    """
    Find circular offsets for n_special copies of the same trace.

    The first node is always offset 0. Only relative offsets matter.

    Objective:
      1. Minimize the maximum aggregate load.
      2. Minimize standard deviation as a tie-breaker.
    """

    T = len(reference)

    # Precompute all possible circular shifts.
    shifts = np.array([np.roll(reference, -offset) for offset in range(T)])

    fixed_nodes = 5 - n_special

    def score(offsets):
        total = shifts[list(offsets)].sum(axis=0)
        total += fixed_nodes * FIXED_RATE

        return (
            total.max(),
            total.std(),
            np.percentile(total, 99),
        ), total

    # --------------------------------------------------------
    # 2 special nodes:
    # only 100 possible offsets.
    # --------------------------------------------------------

    if n_special == 2:
        best = None

        for b in range(T):
            result = score((0, b))

            if best is None or result[0] < best[0]:
                best = (result[0], (0, b), result[1])

        return best[1], best[2]

    # --------------------------------------------------------
    # 3 special nodes:
    # only 100 * 100 = 10,000 combinations.
    # --------------------------------------------------------

    if n_special == 3:
        best = None

        for b in range(T):
            for c in range(T):
                result = score((0, b, c))

                if best is None or result[0] < best[0]:
                    best = (
                        result[0],
                        (0, b, c),
                        result[1],
                    )

        return best[1], best[2]

    # --------------------------------------------------------
    # 4/5 special nodes:
    #
    # Exhaustive search would require:
    #
    #   4 nodes -> 100^3 = 1,000,000
    #   5 nodes -> 100^4 = 100,000,000
    #
    # Use random global search followed by local optimization.
    # --------------------------------------------------------

    rng = np.random.default_rng(42)

    # Start with evenly distributed phases. This is a very good
    # starting point when the trace is approximately sinusoidal.
    best_offsets = [round(i * T / n_special) % T for i in range(n_special)]
    best_offsets[0] = 0

    best_score, best_total = score(best_offsets)

    # Randomly explore the search space.
    for _ in range(20000):
        offsets = [0] + rng.integers(0, T, size=n_special - 1).tolist()

        current_score, current_total = score(offsets)

        if current_score < best_score:
            best_score = current_score
            best_offsets = offsets
            best_total = current_total

    # --------------------------------------------------------
    # Local refinement:
    # optimize one node's offset at a time until no
    # improvement is possible.
    # --------------------------------------------------------

    improved = True
    while improved:
        improved = False

        for node in range(1, n_special):
            current = best_offsets[node]

            for offset in range(T):
                if offset == current:
                    continue

                candidate = best_offsets.copy()
                candidate[node] = offset

                current_score, current_total = score(candidate)

                if current_score < best_score:
                    best_score = current_score
                    best_offsets = candidate
                    best_total = current_total
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

    if "0" not in data:
        raise ValueError("Expected function key '0'.")

    nodes = data["0"]

    # Check that all nodes we want to use actually exist.
    missing = [node for node in SPECIAL_NODES if node not in nodes]

    if missing:
        raise ValueError(f"Missing nodes: {missing}")

    # node_a is the reference trace.
    reference = collapse_trace(nodes[SPECIAL_NODES[0]])

    print(f"Input: {input_file}")
    print(f"Unique samples: {len(reference)}")

    # --------------------------------------------------------
    # Generate configurations with 2, 3, 4 and 5 special nodes.
    # --------------------------------------------------------

    for n_special in range(2, 6):
        special = SPECIAL_NODES[:n_special]

        offsets, total = optimize(reference, n_special)

        print()
        print("=" * 60)
        print(f"{n_special} SPECIAL NODES")
        print("=" * 60)

        for node, offset in zip(special, offsets):
            print(f"{node:10s}: offset = {offset}")

        print()
        print(f"mean: {total.mean():.3f} req/s")
        print(f"std:  {total.std():.3f} req/s")
        print(f"max:  {total.max():.3f} req/s")
        print(f"p95:  {np.percentile(total, 95):.3f} req/s")
        print(f"p99:  {np.percentile(total, 99):.3f} req/s")

        # ----------------------------------------------------
        # Create the output trace.
        # ----------------------------------------------------

        output = {"0": {}}

        for node, original in nodes.items():
            if node in special:
                i = special.index(node)

                shifted = np.roll(reference, -offsets[i])

                output["0"][node] = np.repeat(shifted, 2).tolist()

            else:
                # Fixed nodes remain exactly as they were.
                output["0"][node] = original

        output_file = (
            input_file.parent / f"{input_file.stem}_optimized_{n_special}_special.json"
        )

        with open(output_file, "w") as f:
            json.dump(output, f, indent=2)

        print(f"\nWritten: {output_file}")


if __name__ == "__main__":
    main()
