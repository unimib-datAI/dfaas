#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <input_dir>"
    exit 1
fi

BASE_DIR="$1"

# Python executable. FIXME: make configurable!
PYTHON_ENV="/home/emanuele/ipython-env/env/bin/python"

# Plot scripts. Format: script.py, input CSV, output PDF.
NODE_PLOTS=(
    "cpu_usage_percent_plot.py:mlimage_cpu_usage_percent_1m.csv.gz:mlimage_cpu_usage_percent_1m.pdf"
)

run_node() {
    local node_dir="$1"
    local node_name
    node_name="$(basename "$node_dir")"

    echo "Processing ${node_name}..."

    # Generate plots.
    for entry in "${NODE_PLOTS[@]}"; do
        IFS=":" read -r script input output <<< "${entry}"

        "${PYTHON_ENV}" "${script}" \
            --input "${node_dir}/${input}" \
            --output "${node_dir}/${output}"
    done

    echo "Finished ${node_name}"
}

export -f run_node
export PYTHON_ENV
export NODE_PLOTS

# Run all node_* directories in parallel
for node_dir in "${BASE_DIR}"/node_*; do
    [[ -d "${node_dir}" ]] || continue
    run_node "${node_dir}" &
done

# Wait for all parallel jobs
wait

echo "All node-level processing completed."
