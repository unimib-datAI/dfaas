import argparse
import sys
import subprocess
from pathlib import Path
from concurrent.futures import ThreadPoolExecutor, as_completed


# Node-level plot scripts
NODE_PLOTS = [
    ("k6_forwarded_plot.py", "k6_forwarded.pdf"),
    ("k6_latency_reject_plot.py", "k6_latency_reject.pdf"),
    ("k6_rps_plot.py", "k6_rps.pdf"),
    ("k6_status_code_plot.py", "k6_status_code.pdf"),
    ("k6_rejected_requests_plot.py", "k6_rejected_requests.pdf"),
    ("k6_latency_probability_density_plot.py", "k6_latency_probability_density.pdf"),
    ("k6_served_requests_plot.py", "k6_served_requests_plot.pdf"),
]

# Global plot scripts
GLOBAL_PLOTS = [
    ("k6_status_code_global_plot.py", "k6_status_code.pdf"),
    ("k6_forwarded_global_plot.py", "k6_forwarded.pdf"),
    ("k6_latency_reject_global_plot.py", "k6_latency_reject.pdf"),
    ("k6_rps_global_plot.py", "k6_rps.pdf"),
    (
        "k6_latency_probability_density_global_plot.py",
        "k6_latency_probability_density_global.pdf",
    ),
    ("k6_served_requests_global_plot.py", "k6_served_requests_global_plot.pdf"),
]


def read_iter_duration(base_dir):
    path = base_dir / "iter_duration_seconds"
    if not path.is_file():
        return None

    value = path.read_text().strip()
    return value if value else None


def run_k6_parser(input_csv, output_csv):
    subprocess.run(
        [
            sys.executable,
            "k6_parser.py",
            "--input",
            str(input_csv),
            "--output",
            str(output_csv),
            "--rl-strategy",
        ],
        check=True,
    )


def run_plot(script, input_path, output_path, iter_args):
    subprocess.run(
        [
            sys.executable,
            script,
            "--input",
            str(input_path),
            "--output",
            str(output_path),
            *iter_args,
        ],
        check=True,
    )


def process_node(node_dir, iter_args):
    node_name = node_dir.name
    print(f"[INFO] Processing {node_name}...")

    input_csv = node_dir / "k6_results.csv.gz"
    processed_csv = node_dir / "k6_results_processed.csv"

    if not input_csv.is_file():
        print(f"[WARN] Missing input file: {input_csv} (skipping {node_name})")
        return

    # Step 1: CSV conversion (sequential, blocking)
    run_k6_parser(input_csv, processed_csv)

    # Step 2: Node plots (parallel)
    with ThreadPoolExecutor(max_workers=8) as executor:
        futures = {
            executor.submit(
                run_plot,
                script,
                processed_csv,
                node_dir / output,
                iter_args,
            ): script
            for script, output in NODE_PLOTS
        }

        for future in as_completed(futures):
            script = futures[future]
            try:
                future.result()
            except Exception as e:
                print(f"[ERROR] Plot failed ({script}) in {node_name}: {e}")

    print(f"[INFO] Finished {node_name}")


def main():
    parser = argparse.ArgumentParser(
        description="Generate k6 plots for DFaaS nodes (expects node_* subdirectories)."
    )

    parser.add_argument(
        "input_dir",
        type=Path,
        nargs="+",
        help="One or more input directories containing node_* subdirectories.",
    )

    args = parser.parse_args()

    for base_dir in args.input_dir:
        base_dir = base_dir.resolve()
        print(f"[INFO] Base directory for k6 plots: {base_dir}")

        if not base_dir.is_dir():
            print(f"[ERROR] Not a directory: {base_dir}")
            continue

        iter_duration_file = base_dir / "iter_duration_seconds"
        iter_args = []

        if iter_duration_file.is_file():
            value = iter_duration_file.read_text().strip()
            if value:
                print(f"[INFO] Using iter-duration={value}")
                iter_args = ["--iter-duration", value]

        node_dirs = sorted(d for d in base_dir.glob("node_*") if d.is_dir())

        print("[INFO] Generating plots for each DFaaS node...")

        for node_dir in node_dirs:
            process_node(node_dir, iter_args)

        print("[INFO] All node-level processing completed.")

        # Global plots
        global_dir = base_dir / "global"
        global_dir.mkdir(exist_ok=True)

        print("[INFO] Generating global plots...")

        with ThreadPoolExecutor(max_workers=8) as executor:
            futures = {
                executor.submit(
                    run_plot,
                    script,
                    base_dir,
                    global_dir / output,
                    iter_args,
                ): script
                for script, output in GLOBAL_PLOTS
            }

            for future in as_completed(futures):
                script = futures[future]
                try:
                    future.result()
                except Exception as e:
                    print(f"[ERROR] Global plot failed ({script}): {e}")

        print("[INFO] Global plots completed.")


if __name__ == "__main__":
    main()
