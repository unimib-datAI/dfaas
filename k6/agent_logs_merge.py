#!/usr/bin/env python3
#
# Run with the --help flag for more information.
#
# This script is intended to be run using uv with the following command:
#
#       uv run python agent_merge_logs.py [exp_dir...]
#
import argparse
import json
import re
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path


TIMESTAMP_PATTERN = re.compile(
    r"(\d{4}-\d{2}-\d{2})T"
    r"(\d{2}:\d{2}:\d{2})"
    r"(?:\.(\d+))?"
    r"(Z|[+-]\d{2}:\d{2})?"
)


def timestamp_key(ts, source, line_number):
    # Parse an ISO 8601 timestamp and return a sortable UTC key.
    # Nanosecond precision is preserved without using floating-point values.
    #
    # Timestamps without an explicit timezone are interpreted as UTC.
    if not isinstance(ts, str):
        raise ValueError(f"{source}:{line_number}: Missing or non-string 'ts' field")

    match = TIMESTAMP_PATTERN.fullmatch(ts)

    if not match:
        raise ValueError(f"{source}:{line_number}: Invalid timestamp: {ts!r}")

    date_part, time_part, fraction, tz_string = match.groups()

    try:
        if not tz_string or tz_string == "Z":
            tz = timezone.utc
        else:
            sign = 1 if tz_string[0] == "+" else -1
            hours, minutes = map(int, tz_string[1:].split(":"))

            if hours > 23 or minutes > 59:
                raise ValueError("Invalid timezone offset")

            tz = timezone(sign * timedelta(hours=hours, minutes=minutes))

        dt = datetime.strptime(f"{date_part}T{time_part}", "%Y-%m-%dT%H:%M:%S").replace(
            tzinfo=tz
        )

        utc_dt = dt.astimezone(timezone.utc)

        # Calculate integer seconds since the Unix epoch.
        epoch = datetime(1970, 1, 1, tzinfo=timezone.utc)
        delta = utc_dt - epoch
        seconds = delta.days * 86400 + delta.seconds

        # Normalize the fractional part to nanoseconds.
        nanos = int(((fraction or "") + "000000000")[:9])

        return seconds, nanos

    except (ValueError, OverflowError) as exc:
        raise ValueError(
            f"{source}:{line_number}: Invalid timestamp {ts!r}: {exc}"
        ) from exc


def merge_experiment(exp_dir):
    # Merge all node agent.log files for a single experiment.
    #
    # Example:
    # data/.../experiment/other/node_a/agent.log
    # data/.../experiment/other/node_b/agent.log
    #
    # Output:
    # data/.../experiment/other/global/agent.log

    exp_dir = exp_dir.resolve()
    other_dir = exp_dir / "other"
    global_dir = other_dir / "global"

    if not exp_dir.is_dir():
        raise FileNotFoundError(f"Experiment directory not found: {exp_dir}")

    if not other_dir.is_dir():
        raise FileNotFoundError(f"'other' directory not found: {other_dir}")

    # Find all agent.log files inside node_* directories.
    log_files = sorted(other_dir.glob("node_*/agent.log"))

    if not log_files:
        raise FileNotFoundError(
            f"{exp_dir}: No agent.log files found in node_* directories"
        )

    entries = []

    for log_file in log_files:
        # Extract the node name from the parent directory.
        # Example:
        # .../other/node_a/agent.log -> node_a
        node = log_file.parent.name

        with log_file.open("r", encoding="utf-8") as infile:
            for line_number, line in enumerate(infile, start=1):
                if not line.strip():
                    continue

                # Parse each line as an independent JSON object.
                try:
                    entry = json.loads(line)
                except json.JSONDecodeError as exc:
                    raise ValueError(
                        f"{log_file}:{line_number}: Malformed JSON: {exc}"
                    ) from exc

                if not isinstance(entry, dict):
                    raise ValueError(
                        f"{log_file}:{line_number}: JSON entry must be an object"
                    )

                # Validate the timestamp before accepting the entry.
                sort_key = timestamp_key(entry.get("ts"), log_file, line_number)

                # Add the source node to each log entry.
                entry["node"] = node

                entries.append((sort_key, entry))

    # Sort all entries chronologically across all nodes.
    entries.sort(key=lambda item: item[0])

    global_dir.mkdir(parents=True, exist_ok=True)
    output_file = global_dir / "agent.log"

    # Write one JSON object per line.
    with output_file.open("w", encoding="utf-8") as outfile:
        for _, entry in entries:
            outfile.write(json.dumps(entry, ensure_ascii=False) + "\n")

    print(f"Output JSON merged logs: {output_file}")


def expand_experiments(paths):
    """Expand input paths into a list of experiment directories."""
    experiments = []
    for path in paths:
        if not path.is_dir():
            raise FileNotFoundError(
                f"Input path does not exist or is not a directory: {path}"
            )

        if (path / "other").is_dir():
            # This path is already an experiment directory.
            experiments.append(path)
        else:
            # Search for experiment directories inside the given directory.
            children = sorted(path.iterdir())

            matching = [
                child
                for child in children
                if child.is_dir() and (child / "other").is_dir()
            ]

            if not matching:
                raise FileNotFoundError(
                    f"{path}: No experiment directories containing an 'other' directory were found"
                )

            experiments.extend(matching)

    # Avoid processing the same experiment more than once.
    return list(dict.fromkeys(exp.resolve() for exp in experiments))


def main():
    parser = argparse.ArgumentParser(
        formatter_class=argparse.RawDescriptionHelpFormatter,
        description=(
            "Merge all agent.log files from the different nodes of each "
            "experiment into a single, chronologically sorted JSON Lines file."
        ),
        epilog="""
Expected experiment directory structure:

experiment/
└── other/
    ├── node_a/
    │   └── agent.log
    ├── node_b/
    │   └── agent.log
    └── global/
        └── agent.log  <-- generated output

The generated log contains all entries from all nodes, sorted by timestamp,
with an additional "node" field identifying the source node.

Input can be:
  1. A single experiment directory:
     data/hybrid/v1

  2. A directory containing multiple experiments:
     data/experiments

  3. A mix of both:
     data/hybrid/v1 data/experiments

Each input directory must either be an experiment containing an "other"
directory or a parent directory containing experiment subdirectories.
""",
    )

    parser.add_argument(
        "experiments",
        type=Path,
        nargs="+",
        help="Experiment directories or directories containing multiple experiments.",
    )

    args = parser.parse_args()

    try:
        experiments = expand_experiments(args.experiments)

        for exp_dir in experiments:
            merge_experiment(exp_dir)

    except (ValueError, OSError) as exc:
        print(f"[ERROR] {exc}")
        sys.exit(1)


if __name__ == "__main__":
    main()
