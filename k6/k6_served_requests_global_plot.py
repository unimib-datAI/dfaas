import argparse
from pathlib import Path

import k6_served_requests_plot

import pandas as pd


def load_data(input_dir):
    """Returns a merged DataFrame containing data from all
    'node_*/k6_results_processed.csv' files found within the specified directory
    path."""
    csv_files = list(input_dir.glob("node_*/k6_results_processed.csv"))

    if not csv_files:
        raise FileNotFoundError(
            f"No files found matching node_*/k6_results_processed.csv in {input_dir}"
        )

    dfs = []
    for file in csv_files:
        df = pd.read_csv(file)
        df["node_name"] = file.parent.name
        dfs.append(df)

    return pd.concat(dfs, ignore_index=True)


def main():
    desc = (
        "This script generates a plot showing the success rate of served "
        "HTTP requests over time. It computes whether each request was "
        "successfully served (http_req_failed == 0) and aggregates results "
        "per iteration and per phase. If no phase is found, all data is "
        "treated as a single default phase. "
        "This is the 'global' version, which considers all nodes under the "
        "specified input directory."
    )
    parser = argparse.ArgumentParser(description=desc)

    parser.add_argument(
        "--input",
        required=True,
        help="Path to input directory containing node_* subdirectories",
    )
    parser.add_argument("--output", required=True, type=Path, help="Path to output PDF")
    parser.add_argument("--iter-duration", help="Ignored, kept for compatibility")

    args = parser.parse_args()

    input_path = Path(args.input)

    if not input_path.is_dir():
        raise NotADirectoryError(f"{input_path} is not a valid directory")

    df = load_data(input_path)
    nodes = df["node_name"].unique().tolist()
    print(f"Found the following {len(nodes)} nodes under {input_path}: {nodes}")

    fig = k6_served_requests_plot.plot(df)
    fig.savefig(args.output, bbox_inches="tight")

    print(f"Saved plot to {args.output.as_posix()}")


if __name__ == "__main__":
    main()
