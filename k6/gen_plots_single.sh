#!/usr/bin/env bash

EXP="$1"
if [[ -z "$EXP" ]]; then
    echo "No experiment directory provided"
else
    echo "Exp. dir.: $1"
fi

PYTHON_BIN=/home/emanuele/ipython-env/env/bin/python

$PYTHON_BIN k6_parser.py --input "$EXP/k6/k6_results.csv.gz" --output "$EXP/k6/k6_results_processed.csv"
$PYTHON_BIN -m plots.rps --input "$EXP/k6/k6_results_processed.csv" --output "$EXP/k6/rps.pdf"
$PYTHON_BIN -m plots.latency_reject --input "$EXP/k6/k6_results_processed.csv" --output "$EXP/k6/latency_reject.pdf"
$PYTHON_BIN -m plots.served_requests --input "$EXP/k6/k6_results_processed.csv" --output "$EXP/k6/served_requests.pdf"
