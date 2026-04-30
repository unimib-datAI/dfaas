import argparse

import pandas as pd


def build_request_table(df):
    """Input must be a CSV from k6 output!."""
    # Keep only relevant columns.
    df = df.loc[:, ["timestamp", "metric_name", "metric_value", "status", "extra_tags"]]

    # Keep only relevant rows.
    # ruff: noqa: F841
    keep_metrics = [
        "http_reqs",
        "http_req_duration",
        "http_req_blocked",
        "http_req_connecting",
        "http_req_connecting",
        "http_req_waiting",
        "http_req_receiving",
        "http_req_failed",
        "DFaaS",
    ]
    df = df.query("metric_name in @keep_metrics")

    # We need to set a request ID to allow grouping metrics for a single
    # request. We know that k6 emits metrics ordered, so when we encounter a new
    # "http_reqs" metric we are considering a new request. We can use this
    # assumption to easily generate the "request_id" column.
    mask = df["metric_name"] == "http_reqs"
    df["request_id"] = mask.cumsum()

    # Hack: "DFaaS" rows may not be aligned with "http_reqs", but we know that
    # the number of "DFaaS" rows are the same of "http_reqs" rows, so we can
    # align manually.
    mask = df["metric_name"] == "DFaaS"
    # Modify the "request_id" column only for "DFaaS" rows!
    df.loc[mask, "request_id"] = mask.cumsum()[mask]

    # Now we add a new metric_name called "http_status" with the response HTTP
    # status, extracted from the http_reqs rows. One row for each request.
    http_reqs_rows = df.query("metric_name == 'http_reqs'")
    http_status_rows = http_reqs_rows.copy()  # Required or we modify original data!
    http_status_rows["metric_name"] = "http_status"
    http_status_rows["metric_value"] = http_status_rows["status"]
    df = pd.concat([df, http_status_rows], ignore_index=True)

    # We can drop now the "status" column.
    df = df.drop(columns=["status"])

    # We did the same but for "timestamp".
    http_reqs_rows = df.query("metric_name == 'http_reqs'")
    timestamp_rows = http_reqs_rows.copy()
    timestamp_rows["metric_name"] = "timestamp"
    timestamp_rows["metric_value"] = timestamp_rows["timestamp"]
    df = pd.concat([df, timestamp_rows], ignore_index=True)

    # We can drop now the "timestamp" column.
    df = df.drop(columns=["timestamp"])

    # We add a new metric_name called "k6_stage" with the stage extracted from
    # "http_reqs" rows. One row for each request.
    http_reqs_rows = df.query("metric_name == 'http_reqs'")
    k6_stage_rows = http_reqs_rows.copy()
    k6_stage_rows["metric_name"] = "k6_stage"
    # Here we know and suppose that any http_reqs row have "stage=X" in
    # extra_tags column.
    k6_stage_rows["metric_value"] = (
        k6_stage_rows["extra_tags"].str.extract(r"stage=(\d+)").astype(int)
    )
    df = pd.concat([df, k6_stage_rows], ignore_index=True)

    # We can drop now the rows whose metric_name value is "http_reqs".
    df = df.query("metric_name != 'http_reqs'")

    # We add two new metric_name rows called "dfaas_node_id" and
    # "dfaas_forwarded_to", with values extracted from the row with
    # metric_name=="DFaaS". One for each request.
    dfaas_rows = df.query("metric_name == 'DFaaS'")
    for dfaas_tag in ["DFaaS_Node_ID", "DFaaS_Forwarded_To"]:
        dfaas_tag_rows = dfaas_rows.copy()
        dfaas_tag_rows["metric_name"] = dfaas_tag.lower()

        # Here we know and suppose that any dfaas row have
        # "DFaaS_Node_ID=X&DFaaS_Forwarded_To=X" in extra_tags column.
        dfaas_tag_rows["metric_value"] = dfaas_rows["extra_tags"].str.extract(
            rf"{dfaas_tag}=([^&]*)"
        )
        df = pd.concat([df, dfaas_tag_rows], ignore_index=True)

    # We can drop now the rows whose metric_name value is "DFaaS".
    df = df.query("metric_name != 'DFaaS'")

    # We can drop now the "extra_tags" column.
    df = df.drop(columns=["extra_tags"])

    # We can convert many rows for each request to one row for each request. The
    # new columns names are taken from "metric_name" and values from
    # "metric_value", new index will be "request_id".
    df = df.pivot(
        index="request_id", columns="metric_name", values="metric_value"
    ).reset_index()

    # pivot automatically sets "metric_name" as column index name, we do not
    # want this.
    df.columns.name = None

    # Ensure "k6_stage" is integer. We could not this before since stage was in
    # "metrics_value" column (float).
    df["k6_stage"] = df["k6_stage"].astype(int)

    return df


def split_k6_requests_phase(df):
    df_all_local = df[df["k6_stage"].mod(4).isin([0, 1])]
    df_rl_agent = df[df["k6_stage"].mod(4).isin([2, 3])]

    # We also add "iteration" column that merges 2 k6_stage at time.
    df_all_local["iteration"] = df_all_local["k6_stage"] // 4
    df_rl_agent["iteration"] = df_rl_agent["k6_stage"] // 4

    # Add "phase" column to identify the phase.
    df_all_local["phase"] = "all_local"
    df_rl_agent["phase"] = "rl_agent"

    return df_all_local, df_rl_agent


def main():
    parser = argparse.ArgumentParser(
        description="Parse k6 results into reconstructed metrics CSV."
    )

    parser.add_argument("--input", required=True, help="Path to k6_results.csv.gz")
    parser.add_argument("--output", required=True, help="Path to output CSV file")
    parser.add_argument(
        "--rl-strategy",
        action="store_true",
        help="Enable RL strategy processing (use if the target DFaaS node use RL strategy)",
    )

    args = parser.parse_args()

    df = pd.read_csv(args.input)

    df = build_request_table(df)

    if args.rl_strategy:
        df_all_local, df_rl_agent = split_k6_requests_phase(df)
        df = pd.concat([df_all_local, df_rl_agent], ignore_index=True)

    df.to_csv(args.output, index=False)


if __name__ == "__main__":
    main()
