#!/usr/bin/env python3
# SPDX-License-Identifier: AGPL-3.0-or-later.
# Copyright 2026 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.
#
# Compare target CSV vs source CSV (global avg) per stage.
#
# Fixed schema:
# - Target CSV columns: stage, avg_reqs_per_sec
# - Source CSV columns include: stage, mean_avg_req_per_s
import argparse

import polars as pl

STAGE_COL = "stage"
TARGET_COL = "avg_reqs_per_sec"
SOURCE_COL = "mean_avg_req_per_s"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--target", required=True, help="Path to target CSV (stage, avg_reqs_per_sec)"
    )
    parser.add_argument(
        "--source",
        required=True,
        help="Path to source CSV (must include mean_avg_req_per_s)",
    )
    parser.add_argument("--output", help="Optional path to write the comparison CSV")
    args = parser.parse_args()

    target = pl.read_csv(args.target)
    source = pl.read_csv(args.source)

    missing_target = [c for c in (STAGE_COL, TARGET_COL) if c not in target.columns]
    missing_source = [c for c in (STAGE_COL, SOURCE_COL) if c not in source.columns]
    if missing_target:
        raise SystemExit(
            f"Target CSV missing columns {missing_target}. Found: {list(target.columns)}"
        )
    if missing_source:
        raise SystemExit(
            f"Source CSV missing columns {missing_source}. Found: {list(source.columns)}"
        )

    target = target.with_columns(pl.col(STAGE_COL).cast(pl.Int64, strict=True))
    source = source.with_columns(pl.col(STAGE_COL).cast(pl.Int64, strict=True))

    target = target.select(
        [pl.col(STAGE_COL), pl.col(TARGET_COL).alias("target_avg_reqs_per_sec")]
    )
    source = source.select(
        [pl.col(STAGE_COL), pl.col(SOURCE_COL).alias("calculated_avg_reqs_per_sec")]
    )

    merged = target.join(source, on=STAGE_COL, how="full").sort(STAGE_COL)

    result = (
        merged.with_columns(
            [
                (
                    pl.col("calculated_avg_reqs_per_sec")
                    - pl.col("target_avg_reqs_per_sec")
                ).alias("diff"),
            ]
        )
        .with_columns([pl.col("diff").abs().alias("diff_abs")])
        .select(
            [
                STAGE_COL,
                "target_avg_reqs_per_sec",
                "calculated_avg_reqs_per_sec",
                "diff",
                "diff_abs",
            ]
        )
    )

    # Print table and (optionally) save CSV.
    result.show(limit=None)
    if args.output:
        result.write_csv(args.output)
        print(f"CSV file saved to {args.output}")


if __name__ == "__main__":
    main()
