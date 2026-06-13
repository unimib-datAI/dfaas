#!/usr/bin/env bash
# vim: set tabstop=2 shiftwidth=2 softtabstop=2 expandtab:

set -euo pipefail
shopt -s nullglob

if [[ $# -ne 1 ]]; then
    echo "Usage: $0 <input_dir>"
    exit 1
fi

BASE_DIR="$(realpath "$1")"

echo "[INFO] Base directory for k6 plots: $BASE_DIR..."

# Python executables. FIXME: remove hardcoded paths!
PARSER_PY="/home/emanuele/ipython-env/env/bin/python"
PYTHON="python"

# Plot scripts
NODE_PLOTS=(
    "k6_forwarded_plot.py:k6_forwarded.pdf"
    "k6_latency_reject_plot.py:k6_latency_reject.pdf"
    "k6_rps_plot.py:k6_rps.pdf"
    "k6_status_code_plot.py:k6_status_code.pdf"
    "k6_rejected_requests_plot.py:k6_rejected_requests.pdf"
    "k6_rejected_requests_plot.py:k6_rejected_requests.pdf"
    "k6_latency_probability_density_plot.py:k6_latency_probability_density.pdf"
)

GLOBAL_PLOTS=(
    "k6_status_code_global_plot.py:k6_status_code.pdf"
    "k6_forwarded_global_plot.py:k6_forwarded.pdf"
    "k6_latency_reject_global_plot.py:k6_latency_reject.pdf"
    "k6_rps_global_plot.py:k6_rps.pdf"
    "k6_latency_reject_global_plot.py:k6_latency_reject_global.pdf"
)

run_node() {
  local node_dir="$1"
  local node_name
  node_name="$(basename "$node_dir")"

  echo "[INFO] Processing ${node_name}..."

  local input_csv="${node_dir}/k6_results.csv.gz"
  local processed_csv="${node_dir}/k6_results_processed.csv"

  # Step 1: Pre-process the k6's output.
  if [[ -f "$input_csv" ]]; then
      "${PARSER_PY}" k6_parser.py \
          --input "${input_csv}" \
          --output "${processed_csv}" \
          --rl-strategy
  else
      echo "[WARN] Missing input file: $input_csv (skipping $node_name)"
      return
  fi

  # Step 2: Generate node-level plots.
  for entry in "${NODE_PLOTS[@]}"; do
      IFS=":" read -r script output <<< "${entry}"

      "${PYTHON}" "${script}" \
          --input "${processed_csv}" \
          --output "${node_dir}/${output}"
  done

  echo "[INFO] Finished ${node_name}"
}

export PARSER_PY
export PYTHON

echo "[INFO] Generating plots for each DFaaS node..."

# Run all node_* directories in parallel
for node_dir in "${BASE_DIR}"/node_*; do
    [[ -d "${node_dir}" ]] || continue
    run_node "${node_dir}" &
done

# Wait for all parallel jobs
wait

echo "[INFO] All node-level processing completed."

# Create global output directory
GLOBAL_DIR="${BASE_DIR}/global"
mkdir -p "${GLOBAL_DIR}"

echo "[INFO] Generating global plots..."

# Generate global plots
for entry in "${GLOBAL_PLOTS[@]}"; do
    IFS=":" read -r script output <<< "${entry}"

    "${PYTHON}" "${script}" \
        --input "${BASE_DIR}" \
        --output "${GLOBAL_DIR}/${output}"
done

echo "[INFO] Global plots completed."
