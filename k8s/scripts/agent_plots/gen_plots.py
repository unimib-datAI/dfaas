import argparse
import subprocess
import sys
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path

# Plot scripts. Format: (script.py, input CSV, output PDF).
NODE_PLOTS = [
    ("action_plot.py", "action.csv", "action.pdf"),
]


def run_node(node_dir):
    node_name = node_dir.name
    print(f"Processing {node_name}...")

    input_jsonl = node_dir / "rl_model.log"
    output_observation_csv = node_dir / "observation.csv"
    output_action_csv = node_dir / "action.csv"

    if not input_jsonl.is_file():
        raise FileNotFoundError(f"Missing input file: {input_jsonl}")

    # Step 1: convert JSONL to CSVs.
    subprocess.run(
        [
            sys.executable,
            "parse_rl_model_log.py",
            "--input",
            str(input_jsonl),
            "--obs-output",
            str(output_observation_csv),
            "--act-output",
            str(output_action_csv),
        ],
        check=True,
    )

    # Step 2: generate plots.
    for script, input_file, output_file in NODE_PLOTS:
        subprocess.run(
            [
                sys.executable,
                script,
                "--input",
                str(node_dir / input_file),
                "--output",
                str(node_dir / output_file),
            ],
            check=True,
        )

    print(f"Finished {node_name}")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("input_dir", type=Path, nargs="+")
    args = parser.parse_args()

    for input_dir in args.input_dir:
        base_dir = input_dir.resolve()
        print(f"Base dir: {base_dir.as_posix()}")

        if not base_dir.is_dir():
            print(
                f"Input directory does not exist or is not a directory: {base_dir.as_posix()}"
            )
            return

        node_dirs = sorted(d for d in base_dir.glob("node_*") if d.is_dir())

        errors = False
        with ThreadPoolExecutor() as executor:
            futures = {
                executor.submit(run_node, node_dir): node_dir for node_dir in node_dirs
            }

            for future in as_completed(futures):
                node_dir = futures[future]

                try:
                    future.result()
                except Exception as e:
                    print(f"Error processing {node_dir.name}: {e}")
                    errors = True

        if errors:
            print("A node failed.")
        else:
            print("All node-level processing completed.")


if __name__ == "__main__":
    main()
