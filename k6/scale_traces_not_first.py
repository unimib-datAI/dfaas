#!/usr/bin/env python3
#
# Small script used to scale down traces for nodes from ID 1 to 9, while leaving
# instact the trace for node 0. This to have a load test where only node 0 can
# be overloaded while other nodes can receive forwarding traffic from other
# nodes.
#
# Example usages:
#
#   python modify_nodes.py \
#    -i scaled_pwr_5.json \
#    -o scaled_pwr_5_modified.json \
#    -d 5.5
#
#   python modify_nodes.py \
#    -i scaled_pwr_5.json \
#    -o out.json \
#    -d 6 \
#    --min 0

import json
import argparse
import numpy as np


def process_json(input_file, output_file, target_diff=5.5, min_value=0.0):
    # -----------------------------
    # Load JSON
    # -----------------------------
    with open(input_file, "r") as f:
        data = json.load(f)

    # Assuming only one top-level function key
    func_key = list(data.keys())[0]
    nodes = data[func_key]

    # -----------------------------
    # Compute node 0 mean
    # -----------------------------
    node0 = np.array(nodes["0"], dtype=float)
    node0_mean = node0.mean()

    print(f"\nFunction: {func_key}")
    print(f"Node 0 mean (unchanged): {node0_mean:.3f}")

    stats_before = {}
    stats_after = {}

    # -----------------------------
    # Modify nodes 1..9
    # -----------------------------
    for node_id in map(str, range(1, 10)):

        values = np.array(nodes[node_id], dtype=float)

        old_mean = values.mean()

        # Desired target mean
        target_mean = node0_mean - target_diff

        # Shift required
        shift = old_mean - target_mean

        # Apply shift
        new_values = values - shift

        # Enforce minimum value
        new_values = np.clip(new_values, min_value, None)

        # Save back
        nodes[node_id] = new_values.tolist()

        stats_before[node_id] = old_mean
        stats_after[node_id] = new_values.mean()

    # -----------------------------
    # Save JSON
    # -----------------------------
    with open(output_file, "w") as f:
        json.dump(data, f, indent=2)

    # -----------------------------
    # Print summary
    # -----------------------------
    print("\n=== Summary ===")

    for node_id in map(str, range(1, 10)):
        before = stats_before[node_id]
        after = stats_after[node_id]
        diff = node0_mean - after

        print(
            f"Node {node_id}: "
            f"before={before:.3f}  "
            f"after={after:.3f}  "
            f"diff_from_node0={diff:.3f}"
        )

    print(f"\nSaved modified JSON to: {output_file}")


def main():
    parser = argparse.ArgumentParser(
        description="Lower node values relative to node 0 mean."
    )

    parser.add_argument(
        "-i", "--input",
        required=True,
        help="Input JSON file"
    )

    parser.add_argument(
        "-o", "--output",
        required=True,
        help="Output JSON file"
    )

    parser.add_argument(
        "-d", "--diff",
        type=float,
        default=5.5,
        help="Desired mean difference from node 0 (default: 5.5)"
    )

    parser.add_argument(
        "--min",
        type=float,
        default=0.0,
        dest="min_value",
        help="Minimum allowed value (default: 0)"
    )

    args = parser.parse_args()

    process_json(
        input_file=args.input,
        output_file=args.output,
        target_diff=args.diff,
        min_value=args.min_value
    )


if __name__ == "__main__":
    main()
