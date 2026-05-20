import json
import argparse

import pandas as pd


def load_jsonl(path):
    obs_rows = []
    act_rows = []
    timestamps = []

    with open(path, "r") as f:
        for i, line in enumerate(f):
            record = json.loads(line)

            # timestamp
            ts = record.get("timestamp", i)
            timestamps.append(ts)

            # -------------------
            # OBSERVATIONS
            # -------------------
            # structure: observation -> observation -> node_id -> metrics
            obs_dict = record["observation"]["observation"]

            # flatten: assume single node per line (as in your example)
            node_data = next(iter(obs_dict.values()))
            obs_rows.append(node_data)

            # -------------------
            # ACTIONS
            # -------------------
            # structure: action -> node_id -> actions
            act_dict = record["action"]
            node_action = next(iter(act_dict.values()))
            act_rows.append(node_action)

    # Build DataFrames
    observations = pd.DataFrame(obs_rows)
    actions = pd.DataFrame(act_rows)

    # add timestamp index
    observations.index = range(len(observations))
    actions.index = range(len(actions))

    # optionally keep timestamps as a column
    observations["timestamp"] = timestamps
    actions["timestamp"] = timestamps

    return observations, actions


def main():
    parser = argparse.ArgumentParser(
        description="Parse JSONL logs into observations/actions CSVs."
    )

    parser.add_argument("--input", required=True, help="Path to JSONL file")
    parser.add_argument("--obs-output", required=True, help="Path to observations CSV")
    parser.add_argument("--act-output", required=True, help="Path to actions CSV")

    args = parser.parse_args()

    observations, actions = load_jsonl(args.input)

    observations.to_csv(args.obs_output, index=False)
    actions.to_csv(args.act_output, index=False)

    print(f"Observations saved to: {args.obs_output}")
    print(f"Actions saved to: {args.act_output}")


if __name__ == "__main__":
    main()
