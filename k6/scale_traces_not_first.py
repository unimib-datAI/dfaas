#!/usr/bin/env python3

import json
import argparse


def main():
    parser = argparse.ArgumentParser(
        description=(
            "Modify a trace JSON file:\n"
            "- node '0' keeps the original trace\n"
            "- each trace value is duplicated (100 -> 200 samples)\n"
            "- nodes '1'..'9' are replaced with constant value 5"
        ),
        formatter_class=argparse.RawTextHelpFormatter,
    )

    parser.add_argument("input", help="Input JSON file")
    parser.add_argument("output", help="Output JSON file")

    args = parser.parse_args()

    with open(args.input) as f:
        data = json.load(f)

    # Get original trace of node "0"
    trace0 = data["0"]["0"]

    # Duplicate each item:
    # [1, 2, 3] -> [1, 1, 2, 2, 3, 3]
    # This changes the trace length from 100 to 200
    trace0 = [x for x in trace0 for _ in range(2)]

    # Create new traces:
    # - node "0" keeps the duplicated original trace
    # - nodes "1".."9" contain only constant value 5
    data["0"] = {"0": trace0, **{str(i): [5] * len(trace0) for i in range(1, 10)}}

    # Save modified JSON
    with open(args.output, "w") as f:
        json.dump(data, f, indent=2)


if __name__ == "__main__":
    main()
