#!/usr/bin/env python3

import json
import argparse


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Modify a trace JSON file:\n"
            "- node '0' keeps the original trace (duplicated)\n"
            "- nodes '1'..'9' are replaced with a constant value\n"
        ),
        formatter_class=argparse.RawTextHelpFormatter,
    )

    parser.add_argument("input", help="Input JSON file")
    parser.add_argument("output", help="Output JSON file")

    parser.add_argument(
        "--fixed-value",
        type=float,
        default=4,
        help="Constant value used for nodes 1..9 (default: 4)",
    )

    args = parser.parse_args()

    with open(args.input) as f:
        data = json.load(f)

    # Get original trace of node "0".
    trace0 = data["0"]["0"]

    # Duplicate each item (100 -> 200 samples).
    trace0 = [x for x in trace0 for _ in range(2)]

    # Always use nodes 1..9.
    nodes = [str(i) for i in range(1, 10)]

    # Build constant traces.
    fixed_traces = {node: [args.fixed_value] * len(trace0) for node in nodes}

    # Rebuild node "0".
    data["0"] = {"0": trace0, **fixed_traces}

    with open(args.output, "w") as f:
        json.dump(data, f, indent=2)


if __name__ == "__main__":
    main()
