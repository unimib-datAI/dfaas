import argparse
import sys
from pathlib import Path

import pandas as pd


def main():
    parser = argparse.ArgumentParser(
        description="Prints start and end time from a k6 CSV metrics file."
    )
    parser.add_argument(
        "metrics_file",
        type=Path,
        help="Path to the CSV metrics file (e.g., k6_results.csv.gz)",
    )
    args = parser.parse_args()

    df = pd.read_csv(args.metrics_file, usecols=["timestamp"])

    df["dt"] = pd.to_datetime(df["timestamp"], unit="s", errors="coerce")
    if df["dt"].isnull().all():
        print("Could not find any valid timestamp data.", file=sys.stderr)
        return 1

    start = df["dt"].min()
    end = df["dt"].max()

    fmt = "%Y-%m-%dT%H:%M:%S"
    print("Start:", start.strftime(fmt))
    print("End:", end.strftime(fmt))


if __name__ == "__main__":
    sys.exit(main())
