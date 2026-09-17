set -euo pipefail

run_k6_test() {
    local exp="$1"
    local ip_addr="$2"

    echo "Running k6..., exp $exp"
    script --quiet --command "
    K6_WEB_DASHBOARD=false k6 run single_trace.js --address \"\" \
      --quiet \
      --no-color \
      --out csv=\"$exp/k6/k6_results.csv.gz\" \
      --env IP_SERVER=\"$ip_addr\" \
      --env TRACE_PATH=\"/home/emanuele/dfaas/data/input_requests/mlimage/high_only_e_other_fixed_5_and_9.json\" \
      --env FUNCTION=0 \
      --env NODE=\"node_e\" \
      --env FUNCTION_NAME=\"mlimage\"
    " "$exp/k6/k6_console.logs"

    sleep 5s

    if [[ "${PIPESTATUS[0]}" -eq 0 ]]; then
        ./take_prometheus_snapshot.sh "$ip_addr" "$exp/prom"
    else
        echo "k6 failed; skipping Prometheus snapshot"
        return 1
    fi
}

IP_ADDR="dfaas-node-d.local"

EXP="/home/emanuele/dfaas/data/20260722_node_d_all_local_tests/20260722_node_d_all_local_1"
run_k6_test "$EXP" "$IP_ADDR"
sleep 10m

EXP="/home/emanuele/dfaas/data/20260722_node_d_all_local_tests/20260722_node_d_all_local_2"
run_k6_test "$EXP" "$IP_ADDR"
sleep 10m

EXP="/home/emanuele/dfaas/data/20260722_node_d_all_local_tests/20260722_node_d_all_local_3"
run_k6_test "$EXP" "$IP_ADDR"
sleep 10m

EXP="/home/emanuele/dfaas/data/20260722_node_d_all_local_tests/20260722_node_d_all_local_4"
run_k6_test "$EXP" "$IP_ADDR"
sleep 10m

EXP="/home/emanuele/dfaas/data/20260722_node_d_all_local_tests/20260722_node_d_all_local_5"
run_k6_test "$EXP" "$IP_ADDR"

EXP="/home/emanuele/dfaas/data/20260722_node_d_all_local_tests/20260722_node_d_all_local_6"
run_k6_test "$EXP" "$IP_ADDR"
