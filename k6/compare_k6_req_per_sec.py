#!/usr/bin/env python3
#
# Compare target CSV vs source CSV (global avg) per stage.
#
# Fixed schema:
# - Target CSV columns: stage, avg_reqs_per_sec
# - Source CSV columns include: stage, mean_avg_req_per_s
#
# Behavior:
# - Always prints the per-stage comparison to stdout
# - Optionally saves the comparison to --output (CSV) if provided

import argparse
import pandas as pd

STAGE_COL = "stage"
TARGET_COL = "avg_reqs_per_sec"
SOURCE_COL = "mean_avg_req_per_s"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--target", required=True, help="Path to target CSV (stage, avg_reqs_per_sec)")
    parser.add_argument("--source", required=True, help="Path to source CSV (must include mean_avg_req_per_s)")
    parser.add_argument("--output", help="Optional path to write the comparison CSV")
    args = parser.parse_args()

    target = pd.read_csv(args.target)
    source = pd.read_csv(args.source)

    missing_target = [c for c in (STAGE_COL, TARGET_COL) if c not in target.columns]
    missing_source = [c for c in (STAGE_COL, SOURCE_COL) if c not in source.columns]
    if missing_target:
        raise SystemExit(f"Target CSV missing columns {missing_target}. Found: {list(target.columns)}")
    if missing_source:
        raise SystemExit(f"Source CSV missing columns {missing_source}. Found: {list(source.columns)}")

    target[STAGE_COL] = pd.to_numeric(target[STAGE_COL], errors="raise").astype("int64")
    source[STAGE_COL] = pd.to_numeric(source[STAGE_COL], errors="raise").astype("int64")

    target = target[[STAGE_COL, TARGET_COL]].rename(columns={TARGET_COL: "target_avg_reqs_per_sec"})
    source = source[[STAGE_COL, SOURCE_COL]].rename(columns={SOURCE_COL: "calculated_avg_reqs_per_sec"})

    merged = (
        target.merge(source, on=STAGE_COL, how="outer")
        .sort_values(STAGE_COL, kind="stable")
        .reset_index(drop=True)
    )

    merged["diff"] = merged["calculated_avg_reqs_per_sec"] - merged["target_avg_reqs_per_sec"]
    merged["diff_abs"] = merged["diff"].abs()

    result = merged[
        [
            STAGE_COL,
            "target_avg_reqs_per_sec",
            "calculated_avg_reqs_per_sec",
            "diff",
            "diff_abs",
        ]
    ]

    print(result.to_string(index=False, float_format=lambda x: f"{x:,.2f}"))

    if args.output:
        result.to_csv(args.output, index=False)


if __name__ == "__main__":
    main()
