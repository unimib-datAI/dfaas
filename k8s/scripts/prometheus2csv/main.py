#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later.
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# A small CLI utility for exporting Prometheus metrics or queries to CSV over a
# specified time range.
#
# Run the script with the --help flag for more details.

import csv
import gzip
import argparse
import sys
from datetime import datetime, timedelta
from pathlib import Path

from prometheus_api_client import PrometheusConnect


def stream_to_csv(prom, metric_queries, start_time, end_time, step, output_file):
    """
    Stream Prometheus metrics to gzipped CSV file. Processes one metric at a
    time.

    Args:
        prom: PrometheusConnect instance.
        metric_queries: List of tuples (query, metric_name, entry_type).
        start_time: Start datetime.
        end_time: End datetime.
        step: Query resolution step.
        output_file: Output file path.
    """
    output_path = Path(output_file)

    # Ensure parent directory exists.
    output_path.parent.mkdir(parents=True, exist_ok=True)

    # Fieldnames in the requested order with semicolon separator.
    fieldnames = ["metric", "type", "timestamp", "labels", "value"]
    rows_written = 0

    with gzip.open(output_path, "wt", newline="", encoding="utf-8") as f:
        writer = csv.DictWriter(f, fieldnames=fieldnames, delimiter=";")
        writer.writeheader()

        for metric_idx, (query, metric_name, entry_type) in enumerate(metric_queries):
            print(
                f"Processing {entry_type} {metric_idx + 1}/{len(metric_queries)}: {metric_name}"
            )
            print(f"  Query: {query}")

            try:
                metric_data = prom.custom_query_range(
                    query=query, start_time=start_time, end_time=end_time, step=step
                )
            except Exception as e:
                print(f"Error querying metric '{metric_name}': {e}")
                continue

            if not metric_data:
                print(f"  No data returned for query: {query}")
                continue

            # Stream each series.
            for series in metric_data:
                # Use the provided metric_name instead of __name__ from results.
                labels = {k: v for k, v in series["metric"].items() if k != "__name__"}

                # Format labels as key=value pairs.
                labels_str = ",".join(f"{k}={v}" for k, v in sorted(labels.items()))

                # Stream values one by one.
                for timestamp, value in series["values"]:
                    row = {
                        "metric": metric_name,
                        "type": entry_type,
                        "timestamp": datetime.fromtimestamp(
                            float(timestamp)
                        ).isoformat(),
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
    """
    Load metrics from a CSV file with format: type;query;metric_name;comment.

    The first line is always skipped as it contains the header.

    Returns:
        List of tuples: [(query, metric_name, entry_type), ...].
    """
    metric_queries = []
    with Path(filepath).open("r", encoding="utf-8") as f:
        csv_reader = csv.reader(f, delimiter=";")

        next(csv_reader, None)  # Skip the first line (header).

        # Start at line 2 since line 1 is the header.
        for line_num, row in enumerate(csv_reader, start=2):
            if not row or not any(field.strip() for field in row):
                # Skip empty rows.
                continue

            # Parse CSV columns.
            if len(row) < 2:
                raise ValueError(
                    f"Line {line_num}: Invalid format. Expected at least 2 columns (type;query), got {len(row)}."
                )

            # Column 1: type (metric or query).
            entry_type = row[0].strip()
            if entry_type not in ["metric", "query"]:
                raise ValueError(
                    f"Line {line_num}: Invalid type '{row[0]}'. Must be 'metric' or 'query'."
                )

            # Column 2: query/metric (required).
            query = row[1].strip()
            if not query:
                raise ValueError(
                    f"Line {line_num}: Query/metric column cannot be empty."
                )

            # Column 3: metric_name (optional for metric, required for query).
            metric_name = row[2].strip() if len(row) > 2 else ""

            # Column 4: comment (optional, not used).

            # Queries must always have the metric_name column.
            if not metric_name:
                if entry_type == "query":
                    raise ValueError(
                        f"Line {line_num}: Query type requires a metric name in column 3.\n"
                        f"  Query: {query}\n"
                        f"  Format: query;<your_query>;<metric_name>[;<comment>]."
                    )
                else:
                    # For simple metrics, use query as metric_name.
                    metric_name = query

            metric_queries.append((query, metric_name, entry_type))

    return metric_queries


def parse_datetime_arg(date_str):
    """Parse datetime string in various formats."""
    if not date_str:
        return None

    # Try ISO format with timezone.
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
        f"Unable to parse datetime: {date_str}. Use format like '2026-01-27 12:00:00' or '2026-01-27'."
    )


def parse_args():
    """Parse command-line arguments."""
    parser = argparse.ArgumentParser(
        description="Export Prometheus metrics/queries to gzipped CSV format.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Export single metric (last minute if no time specified).
  %(prog)s -m up
  
  # Export with specific time range.
  %(prog)s -m up -s "2026-01-27 00:00:00" -e "2026-01-28 00:00:00"
  
  # Export multiple metrics.
  %(prog)s -m up -m 'http_requests_total{job="api"}' --step 1m
  
  # Read metrics from CSV file.
  %(prog)s --metrics-file metrics.csv -s "2026-01-27" -e "2026-01-28"
  
  # Provide custom name for query.
  %(prog)s -q "rate(http_requests_total[5m])" -n "http_request_rate"

Metrics File Format:
  The metrics file should be in CSV format with semicolon (;) delimiter.
  The first line is always treated as a header and skipped.
  
  Format: type;query;metric_name;comment.
  
  - type: "metric" or "query" (required).
  - query: Prometheus query or metric name (required).
  - metric_name: Name to use in output (optional for metric, required for query).
  - comment: Description (optional).
  
  Example:
    type;query;metric_name;comment.
    metric;up;;Simple up metric.
    metric;container_memory_usage_bytes;;.
    query;100 * irate(container_cpu_usage_seconds_total[1m]) / on(instance) group_left machine_cpu_cores;cpu_usage_percent;CPU usage percentage.

Output CSV Format:
  The output CSV file uses semicolon (;) as delimiter with columns:
  metric;type;timestamp;labels;value.
  
  - metric: The metric name (from input or specified name).
  - type: "metric" or "query" (original type from input).
  - timestamp: ISO format timestamp.
  - labels: Comma-separated key=value pairs.
  - value: The metric value.
        """,
    )

    # Prometheus connection.
    parser.add_argument(
        "--host", default="localhost", help="Prometheus host (default: localhost)."
    )
    parser.add_argument(
        "--port", type=int, default=9090, help="Prometheus port (default: 9090)."
    )

    # Metrics and time range.
    parser.add_argument(
        "-m",
        "--metric",
        dest="metrics",
        action="append",
        help="Simple metric name (can be specified multiple times). "
        'e.g., "up" or "http_requests_total{job=\\"api\\"}".',
    )
    parser.add_argument(
        "-q",
        "--query",
        dest="queries",
        action="append",
        help="Complex PromQL query (can be specified multiple times). "
        "Must be paired with --name flag.",
    )
    parser.add_argument(
        "-n",
        "--name",
        dest="names",
        action="append",
        help="Metric name for the corresponding --query. "
        "Must be specified for each --query flag.",
    )
    parser.add_argument(
        "--metrics-file",
        type=Path,
        help="CSV file containing metric queries (format: type;query;metric_name;comment). "
        "First line is treated as header and skipped.",
    )
    parser.add_argument(
        "-s",
        "--start",
        help='Start time (format: "2026-01-27 12:00:00" or "2026-01-27"). '
        "Default: 1 minute ago.",
    )
    parser.add_argument(
        "-e",
        "--end",
        help='End time (format: "2026-01-27 12:00:00" or "2026-01-27"). Default: now.',
    )
    parser.add_argument(
        "--step",
        default="15s",
        help="Query resolution step (default: 15s). Examples: 1s, 30s, 1m, 5m, 1h.",
    )

    # Output options.
    parser.add_argument(
        "-o",
        "--output",
        type=Path,
        default=Path("prometheus_metrics.csv.gz"),
        help="Output file path (default: prometheus_metrics.csv.gz).",
    )

    args = parser.parse_args()

    # Validate that at least one metric source is provided.
    if not args.metrics and not args.queries and not args.metrics_file:
        parser.error(
            "At least one of --metric, --query, or --metrics-file must be specified."
        )

    # Validate that --name count matches --query count if both are provided.
    if args.queries:
        if not args.names:
            parser.error("--query requires corresponding --name flag(s).")
        if len(args.names) != len(args.queries):
            parser.error(
                f"Number of --name arguments ({len(args.names)}) must match "
                f"number of --query arguments ({len(args.queries)})."
            )

    # Validate that --name is not used without --query.
    if args.names and not args.queries:
        parser.error("--name can only be used with --query.")

    return args


def main():
    args = parse_args()

    # Construct Prometheus URL.
    prometheus_url = f"http://{args.host}:{args.port}"

    # Collect metrics to export as (query, metric_name, entry_type) tuples.
    metric_queries = []

    # Process command-line metrics (simple metrics).
    if args.metrics:
        for metric in args.metrics:
            metric_queries.append((metric, metric, "metric"))

    # Process command-line queries (complex queries with names).
    if args.queries:
        for query, name in zip(args.queries, args.names):
            metric_queries.append((query, name, "query"))

    # Process metrics file.
    if args.metrics_file:
        try:
            file_metrics = load_metrics_from_file(args.metrics_file)
            metric_queries.extend(file_metrics)
            print(f"Loaded {len(file_metrics)} metrics from {args.metrics_file}")
        except ValueError as e:
            print(f"Error parsing metrics file: {e}", file=sys.stderr)
            return 1

    # Remove duplicates while preserving order (based on metric_name).
    seen = set()
    unique_queries = []
    for query, name, entry_type in metric_queries:
        if name not in seen:
            seen.add(name)
            unique_queries.append((query, name, entry_type))
    metric_queries = unique_queries

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
        print("Start time must be before end time.")
        return 1

    print(f"Prometheus URL: {prometheus_url}")
    print(f"Metrics/Queries to export: {len(metric_queries)}")
    print(f"Time range: {start_time} to {end_time}")
    print(f"Step: {args.step}")
    print(f"Output file: {args.output}")
    print()

    # Create Prometheus connection.
    prom = PrometheusConnect(url=prometheus_url, disable_ssl=True)

    try:
        stream_to_csv(
            prom=prom,
            metric_queries=metric_queries,
            start_time=start_time,
            end_time=end_time,
            step=args.step,
            output_file=args.output,
        )
        print(f"Successfully exported to {args.output}")
        return 0
    except KeyboardInterrupt:
        print("Export cancelled by user.")
        return 0
    except Exception as e:
        print(f"Export failed: {e}")
        import traceback

        traceback.print_exc()
        return 1


if __name__ == "__main__":
    sys.exit(main())
