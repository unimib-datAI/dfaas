#!/usr/bin/env bash

set -euo pipefail

# Input macro experiment directory (contains multiple experiment runs).
# The directory is expected as the first command-line argument.
MACRO_EXP="${1:-}"

# Important: change this to your Python environment!
PYTHON_BIN=/home/emanuele/ipython-env/env/bin/python

if [[ -z "$MACRO_EXP" ]]; then
    echo "Usage: $0 <macro_experiment_directory>"
    exit 1
fi

# Discover experiment run directories.
#
# Each run is expected to contain a k6 directory with k6 output files:
#   macro/run_1/k6/k6_results.csv.gz -> macro/run_1
mapfile -t EXP_DIRS < <(
    find "$MACRO_EXP" -maxdepth 2 -type d -name k6 | sort | xargs -r -n1 dirname
)

if [[ ${#EXP_DIRS[@]} -eq 0 ]]; then
    echo "No experiment runs found"
    exit 1
else
    echo "Found ${#EXP_DIRS[@]} experiment runs on macro directory $MACRO_EXP:"
    for EXP in "${EXP_DIRS[@]}"; do
        echo "  $EXP"
    done
fi

# Arrays used to collect:
# - generated CSV files for plotting
# - HTTP request failure rates extracted from k6 logs
processed_csvs=()
failed_rates=()

echo "Processing experiment runs..."

# Process each experiment run independently.
for EXP in "${EXP_DIRS[@]}"; do
    echo " -> $(basename "$EXP")"

    # 1. Parse raw k6 compressed CSV output.
    $PYTHON_BIN k6_parser.py \
        --input "$EXP/k6/k6_results.csv.gz" \
        --output "$EXP/k6/k6_results_processed.csv"

    # 2. Store the processed CSV path for later plotting.
    processed_csvs+=("$EXP/k6/k6_results_processed.csv")

    # 3. Extract the failed request percentage from the k6 console log.
    failed_rates+=(
        "$(rg -m1 "http_req_failed" "$EXP/k6/k6_console.logs" |
          sed -E 's/.*: *([0-9.]+)%.*/\1/')"
    )
done

echo "Generating plots..."

$PYTHON_BIN -m plots.multiple_runs.served_requests \
    --input "${processed_csvs[@]}" \
    --output "$MACRO_EXP/served_requests.pdf"

$PYTHON_BIN -m plots.multiple_runs.latency_reject \
    --input "${processed_csvs[@]}" \
    --output "$MACRO_EXP/latency_reject.pdf"

# Create a CSV summary containing HTTP request failure rates per run.
#
# Output example:
#   node,run_1,run_2,run_3
#   abc,0.1,0.0,1.2
echo "Generating $MACRO_EXP/http_req_failed.csv..."

# Extract node identifier from the macro experiment directory name.
node=$(basename "$MACRO_EXP" | sed -nE 's/.*node_([a-zA-Z0-9]+).*/\1/p')

{
    # Header row.
    printf "node"
    for i in "${!failed_rates[@]}"; do
        printf ",run_%d" "$((i+1))"
    done
    echo

    # Data row.
    printf "%s" "$node"
    printf ",%s" "${failed_rates[@]}"
    echo
} > "$MACRO_EXP/http_req_failed.csv"

echo "Done"
