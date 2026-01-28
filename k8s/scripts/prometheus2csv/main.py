#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# A small CLI utility for exporting Prometheus metrics to CSV over a specified
# time range.
#
# Run the script with the --help flag for more details.

import csv
import gzip
import argparse
import sys
from datetime import datetime, timedelta
from pathlib import Path

from prometheus_api_client import PrometheusConnect


def stream_to_csv(prom, metrics, start_time, end_time, step, output_file):
    """
    Stream Prometheus metrics to gzipped CSV file. Processes one metric at a
    time.
    """
    output_path = Path(output_file)

    # Ensure parent directory exists.
    output_path.parent.mkdir(parents=True, exist_ok=True)

    # Fixed fieldnames with labels as key-value pairs
    fieldnames = ["timestamp", "metric", "labels", "value"]
    rows_written = 0

    with gzip.open(output_path, "wt", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames)
        writer.writeheader()

        for metric_idx, metric in enumerate(metrics):
            print(f"Processing metric {metric_idx + 1}/{len(metrics)}: {metric}")

            try:
                metric_data = prom.custom_query_range(
                    query=metric, start_time=start_time, end_time=end_time, step=step
                )
            except Exception as e:
                print(f"Error querying metric '{metric}': {e}")
                continue

            if not metric_data:
                print(f"  No data returned for metric: {metric}")
                continue

            # Stream each series
            for series in metric_data:
                metric_name = series["metric"].get("__name__", metric)
                labels = {k: v for k, v in series["metric"].items() if k != "__name__"}

                # Format labels as key=value pairs.
                labels_str = ",".join(f"{k}={v}" for k, v in sorted(labels.items()))

                # Stream values one by one.
                for timestamp, value in series["values"]:
                    row = {
                        "timestamp": datetime.fromtimestamp(
                            float(timestamp)
                        ).isoformat(),
                        "metric": metric_name,
                        "labels": labels_str,
                        "value": value,
                    }

                    writer.writerow(row)
                    rows_written += 1

        if rows_written == 0:
            print(
                "Warning: No data was written. Check your metrics and time range.",
                file=sys.stderr,
            )
        else:
            print(f"Completed! Total rows written: {rows_written}")


def load_metrics_from_file(filepath):
    """Load metrics from a file, one per line, ignoring empty lines and comments."""
    metrics_path = Path(filepath)

    if not metrics_path.exists():
        raise FileNotFoundError(f"Metrics file not found: {metrics_path}")

    if not metrics_path.is_file():
        raise ValueError(f"Not a file: {metrics_path}")

    metrics = []
    with metrics_path.open("r") as f:
        for line in f:
            line = line.strip()
            if line and not line.startswith("#"):
                metrics.append(line)
    return metrics


def parse_datetime_arg(date_str):
    """Parse datetime string in various formats."""
    if not date_str:
        return None

    # Try ISO format with timezone
    for fmt in [
        "%Y-%m-%dT%H:%M:%S%z",
        "%Y-%m-%dT%H:%M:%S",
        "%Y-%m-%d %H:%M:%S",
        "%Y-%m-%d",
    ]:
        try:
            return datetime.strptime(date_str, fmt)
        except ValueError:
            continue

    raise ValueError(
        f"Unable to parse datetime: {date_str}. Use format like '2026-01-27 12:00:00' or '2026-01-27'"
    )


def parse_args():
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(
        description="Export Prometheus metrics to gzipped CSV format",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Export single metric (last minute if no time specified)
  %(prog)s -m up
  
  # Export with specific time range
  %(prog)s -m up -s "2026-01-27 00:00:00" -e "2026-01-28 00:00:00"
  
  # Export multiple metrics
  %(prog)s -m up -m 'http_requests_total{job="api"}' --step 1m
  
  # Read metrics from file (one per line)
  %(prog)s --metrics-file metrics.txt -s "2026-01-27" -e "2026-01-28"
        """,
    )

    # Prometheus connection.
    parser.add_argument(
        "--host", default="localhost", help="Prometheus host (default: localhost)"
    )
    parser.add_argument(
        "--port", type=int, default=9090, help="Prometheus port (default: 9090)"
    )

    # Metrics and time range.
    parser.add_argument(
        "--metric",
        dest="metrics",
        action="append",
        help="Metric query (can be specified multiple times). "
        'Supports PromQL syntax with labels, e.g., "up" or '
        '"http_requests_total{job=\\"api\\"}"',
    )
    parser.add_argument(
        "--metrics-file",
        type=Path,
        help="Text file containing metric queries (one per line)",
    )
    parser.add_argument(
        "-s",
        "--start",
        help='Start time (format: "2026-01-27 12:00:00" or "2026-01-27"). '
        "Default: 1 minute ago",
    )
    parser.add_argument(
        "-e",
        "--end",
        help='End time (format: "2026-01-27 12:00:00" or "2026-01-27"). Default: now',
    )
    parser.add_argument(
        "--step",
        default="15s",
        help="Query resolution step (default: 15s). Examples: 1s, 30s, 1m, 5m, 1h",
    )

    # Output options.
    parser.add_argument(
        "-o",
        "--output",
        type=Path,
        default=Path("prometheus_metrics.csv.gz"),
        help="Output file path (default: prometheus_metrics.csv.gz)",
    )

    args = parser.parse_args()

    # Validate that at least one metric source is provided.
    if not args.metrics and not args.metrics_file:
        parser.error("At least one of --metric or --metrics-file must be specified")

    return args


def main():
    args = parse_args()

    # Construct Prometheus URL.
    prometheus_url = f"http://{args.host}:{args.port}"

    # Collect metrics to export.
    metrics = args.metrics or []
    if args.metrics_file:
        file_metrics = load_metrics_from_file(args.metrics_file)
        metrics.extend(file_metrics)
        print(f"Loaded {len(file_metrics)} metrics from {args.metrics_file}")

    # Remove duplicates while preserving order.
    metrics = list(dict.fromkeys(metrics))

    # Parse time arguments with defaults.
    if args.end:
        end_time = parse_datetime_arg(args.end)
    else:
        end_time = datetime.now()

    if args.start:
        start_time = parse_datetime_arg(args.start)
    else:
        start_time = end_time - timedelta(minutes=1)

    # Validate time range.
    if start_time >= end_time:
        print("Start time must be before end time")
        return 1

    print(f"Prometheus URL: {prometheus_url}")
    print(f"Metrics to export: {len(metrics)}")
    print(f"Time range: {start_time} to {end_time}")
    print(f"Step: {args.step}")
    print(f"Output file: {args.output}")
    print()

    # Create Prometheus connection.
    prom = PrometheusConnect(url=prometheus_url, disable_ssl=True)

    try:
        stream_to_csv(
            prom=prom,
            metrics=metrics,
            start_time=start_time,
            end_time=end_time,
            step=args.step,
            output_file=args.output,
        )
        print(f"Successfully exported to {args.output}")
        return 0
    except KeyboardInterrupt:
        print("Export cancelled by user")
        return 0
    except Exception as e:
        print(f"Export failed: {e}")
        import traceback

        traceback.print_exc()
        return 1


if __name__ == "__main__":
    sys.exit(main())
