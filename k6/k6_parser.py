# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# This script converts raw k6 output CSV files into a reconstructed metrics
# table, where each request is represented by a single row and the columns
# contain all the metrics and information associated with that request.
#
# The main motivation for this transformation is that the raw k6 output
# contains multiple rows for a single request, which makes the CSV file more
# difficult to process and analyze in later stages.

import argparse

import polars as pl


def build_request_table(df):
    """Input must be a CSV from k6 output!."""
    # Keep only relevant columns.
    df = df.select(["timestamp", "metric_name", "metric_value", "status", "extra_tags"])

    # Keep metric_value as string before the pivot. This column contains both
    # numeric metrics and string values (e.g. DFaaS node identifiers).
    df = df.with_columns(pl.col("metric_value").cast(pl.String))

    # Keep only relevant rows.
    keep_metrics = [
        "http_reqs",
        "http_req_duration",
        "http_req_blocked",
        "http_req_connecting",
        "http_req_waiting",
        "http_req_receiving",
        "http_req_failed",
        "DFaaS",
    ]
    df = df.filter(pl.col("metric_name").is_in(keep_metrics))

    # We need to set a request ID to allow grouping metrics for a single
    # request. We know that k6 emits metrics ordered, so when we encounter a new
    # "http_reqs" metric we are considering a new request. We can use this
    # assumption to easily generate the "request_id" column.
    df = df.with_columns(
        (pl.col("metric_name") == "http_reqs")
        .cum_sum()
        .cast(pl.Int64)
        .alias("request_id")
    )

    # Hack: "DFaaS" rows may not be aligned with "http_reqs", but we know that
    # the number of "DFaaS" rows are the same of "http_reqs" rows, so we can
    # align manually.
    df = df.with_columns(
        pl.when(pl.col("metric_name") == "DFaaS")
        .then((pl.col("metric_name") == "DFaaS").cum_sum())
        .otherwise(pl.col("request_id"))
        .cast(pl.Int64)
        .alias("request_id")
    )

    # Now we add a new metric_name called "http_status" with the response HTTP
    # status, extracted from the http_reqs rows. One row for each request.
    http_reqs_rows = df.filter(pl.col("metric_name") == "http_reqs")
    http_status_rows = http_reqs_rows.with_columns(
        pl.lit("http_status").alias("metric_name"),
        pl.col("status").cast(pl.String).alias("metric_value"),
    )
    df = pl.concat([df, http_status_rows])

    # We can drop now the "status" column.
    df = df.drop("status")

    # We did the same but for "timestamp".
    http_reqs_rows = df.filter(pl.col("metric_name") == "http_reqs")
    timestamp_rows = http_reqs_rows.with_columns(
        pl.lit("timestamp").alias("metric_name"),
        pl.col("timestamp").cast(pl.String).alias("metric_value"),
    )
    df = pl.concat([df, timestamp_rows])

    # We can drop now the "timestamp" column.
    df = df.drop("timestamp")

    # We add a new metric_name called "k6_stage" with the stage extracted from
    # "http_reqs" rows. One row for each request.
    http_reqs_rows = df.filter(pl.col("metric_name") == "http_reqs")
    k6_stage_rows = http_reqs_rows.with_columns(
        pl.lit("k6_stage").alias("metric_name"),
        pl.col("extra_tags").str.extract(r"stage=(\d+)").alias("metric_value"),
    )
    df = pl.concat([df, k6_stage_rows])

    # We can drop now the rows whose metric_name value is "http_reqs".
    df = df.filter(pl.col("metric_name") != "http_reqs")

    # We add two new metric_name rows called "dfaas_node_id" and
    # "dfaas_forwarded_to", with values extracted from the row with
    # metric_name=="DFaaS". One for each request.
    dfaas_rows = df.filter(pl.col("metric_name") == "DFaaS")

    for dfaas_tag in ["DFaaS_Node_ID", "DFaaS_Forwarded_To"]:
        dfaas_tag_rows = dfaas_rows.with_columns(
            pl.lit(dfaas_tag.lower()).alias("metric_name"),
            pl.col("extra_tags")
            .str.extract(rf"{dfaas_tag}=([^&]*)")
            .alias("metric_value"),
        )
        df = pl.concat([df, dfaas_tag_rows])

    # We can drop now the rows whose metric_name value is "DFaaS".
    df = df.filter(pl.col("metric_name") != "DFaaS")

    # We can drop now the "extra_tags" column.
    df = df.drop("extra_tags")

    # We can convert many rows for each request to one row for each request. The
    # new columns names are taken from "metric_name" and values from
    # "metric_value", new index will be "request_id".
    df = df.pivot(
        index="request_id",
        on="metric_name",
        values="metric_value",
    )

    # Ensure "request_id", "timestamp" and "k6_stage" are integers. They were
    # stored in the generic "metric_value" column before the pivot.
    df = df.with_columns(
        pl.col("request_id").cast(pl.Int64),
        pl.col("timestamp").cast(pl.Int64),
        pl.col("k6_stage").cast(pl.Int64),
    )

    # Do the same also for "http_status", but in this case the value may be
    # missing (e.g. request timeout at k6 level, so with no response) and we
    # want to keep these missing values.
    df = df.with_columns(
        pl.col("http_status").cast(pl.Int64, strict=False),
    )

    # Add the "iteration" column that merges 2 k6_stage at time.
    df = df.with_columns((pl.col("k6_stage") // 2).cast(pl.Int64).alias("iteration"))

    # Convert numeric metrics back to numeric types after the pivot.
    for column in [
        "http_req_duration",
        "http_req_blocked",
        "http_req_connecting",
        "http_req_waiting",
        "http_req_receiving",
        "http_req_failed",
    ]:
        if column in df.columns:
            df = df.with_columns(pl.col(column).cast(pl.Float64))

    return df


def main():
    parser = argparse.ArgumentParser(
        description="Parse k6 results into reconstructed metrics CSV."
    )

    parser.add_argument("--input", required=True, help="Path to k6_results.csv.gz")
    parser.add_argument("--output", required=True, help="Path to output CSV file")

    args = parser.parse_args()

    df = pl.read_csv(args.input)

    df = build_request_table(df)

    df.write_csv(args.output)
    print(f"CSV saved to: {args.output}")


if __name__ == "__main__":
    main()
