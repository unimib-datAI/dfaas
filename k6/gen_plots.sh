#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <input_dir>"
    exit 1
fi

BASE_DIR="$1"

# Python executables
PARSER_PY="/home/emanuele/ipython-env/env/bin/python"
PYTHON="python"

# Plot scripts
NODE_PLOTS=(
    "k6_forwarded_plot.py:k6_forwarded.pdf"
    "k6_latency_reject_plot.py:k6_latency_reject.pdf"
    "k6_rps_plot.py:k6_rps.pdf"
    "k6_status_code_plot.py:k6_status_code.pdf"
    "k6_rejected_requests_plot.py:k6_rejected_requests.pdf"
)

GLOBAL_PLOTS=(
    "k6_status_code_global_plot.py:k6_status_code.pdf"
    "k6_forwarded_global_plot.py:k6_forwarded.pdf"
    "k6_latency_reject_global_plot.py:k6_latency_reject.pdf"
    "k6_rps_global_plot.py:k6_rps.pdf"
)

run_node() {
    local node_dir="$1"
    local node_name
    node_name="$(basename "$node_dir")"

    echo "Processing ${node_name}..."

    local input_csv="${node_dir}/k6_results.csv.gz"
    local processed_csv="${node_dir}/k6_results_processed.csv"

    # Step 1: Parse
    "${PARSER_PY}" k6_parser.py \
        --input "${input_csv}" \
        --output "${processed_csv}" \
        --rl-strategy

    # Step 2: Generate node-level plots
    for entry in "${NODE_PLOTS[@]}"; do
        IFS=":" read -r script output <<< "${entry}"

        "${PYTHON}" "${script}" \
            --input "${processed_csv}" \
            --output "${node_dir}/${output}"
    done

    echo "Finished ${node_name}"
}

export -f run_node
export PARSER_PY
export PYTHON
export NODE_PLOTS

# Run all node_* directories in parallel
for node_dir in "${BASE_DIR}"/node_*; do
    [[ -d "${node_dir}" ]] || continue
    run_node "${node_dir}" &
done

# Wait for all parallel jobs
wait

echo "All node-level processing completed."

# Create global output directory
GLOBAL_DIR="${BASE_DIR}/global"
mkdir -p "${GLOBAL_DIR}"

# Generate global plots
for entry in "${GLOBAL_PLOTS[@]}"; do
    IFS=":" read -r script output <<< "${entry}"

    "${PYTHON}" "${script}" \
        --input "${BASE_DIR}" \
        --output "${GLOBAL_DIR}/${output}"
done

echo "Global plots completed."
