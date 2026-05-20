import argparse
from pathlib import Path

import pandas as pd
import matplotlib.pyplot as plt


def plot_rejection_breakdown(df, node_name):
    # The forwarded node label has too many characters, reduce them to last 4.
    def short_node_label(node_id):
        if pd.isna(node_id):
            return None
        return f"node_{str(node_id)[-4:]}"

    df["forwarded_label"] = df["dfaas_forwarded_to"].apply(short_node_label)

    # Rejection flags.
    df["rejected"] = df["http_req_failed"] == 1
    df["direct_403"] = df["rejected"] & (df["http_status"] == 403)
    df["local_5xx"] = (
        df["rejected"]
        & df["http_status"].isin([500, 504])
        & df["dfaas_forwarded_to"].isna()
    )
    df["forwarded_rejected"] = df["rejected"] & df["dfaas_forwarded_to"].notna()

    # Split the "all_local" and "rl_agent" phase in different dataframes.
    def build_table(phase):
        phase_df = df[df["phase"] == phase]

        result = pd.DataFrame()

        direct = phase_df.groupby("iteration")["direct_403"].sum().rename("direct_403")
        local = phase_df.groupby("iteration")["local_5xx"].sum().rename("local_5xx")
        result = pd.concat([direct, local], axis=1)
        forwarded = phase_df[phase_df["forwarded_rejected"]]

        if not forwarded.empty:
            forwarded_counts = (
                forwarded.groupby(["iteration", "forwarded_label"])
                .size()
                .unstack(fill_value=0)
            )
            result = pd.concat([result, forwarded_counts], axis=1)

        return result.fillna(0).sort_index()

    all_local = build_table("all_local")
    rl_agent = build_table("rl_agent")

    # Reorder the columns before plotting and also rename them (from
    # "direct_403" to "Direct" as example).
    def reorder_and_rename(table):
        rename_map = {}
        forwarded_cols = []
        for column in table.columns:
            match column:
                case "direct_403":
                    rename_map[column] = "Direct"
                case "local_5xx":
                    rename_map[column] = "Local"
                case _:
                    rename_map[column] = f"Fwd to {column}"
                    forwarded_cols.append(column)

        ordered_cols = ["direct_403", "local_5xx"] + sorted(forwarded_cols)

        table = table[ordered_cols]
        return table.rename(columns=rename_map)

    all_local = reorder_and_rename(all_local)
    rl_agent = reorder_and_rename(rl_agent)

    # Now we can generate the plot. We must have a shared Y axis to avoid
    # misleading sub-plots.
    fig, (ax1, ax2) = plt.subplots(
        nrows=2, ncols=1, sharey=True, sharex=True, figsize=(14, 9)
    )
    fig.suptitle(f"Rejected requests by {node_name}")

    def plot_phase(ax, table, title):
        x = table.index.values
        y = [table[col].values for col in table.columns]

        ax.stackplot(x, y, labels=table.columns, alpha=0.8)

        ax.set_title(title)
        ax.set_ylabel("Rejected requests")
        ax.grid(visible=True, which="both", alpha=0.3)
        ax.legend(loc="best")

    plot_phase(ax1, all_local, "All Local Phase")
    plot_phase(ax2, rl_agent, "RL Agent Phase")

    ax2.set_xlabel("Iteration")

    plt.tight_layout()

    return fig


def main():
    parser = argparse.ArgumentParser(
        description=(
            "This script generates a single plot showing the causes of rejected"
            " requests for a single DFaaS node. It has two subplots: one for "
            "the All Local strategy and one for the RL agent strategy. The "
            "causes of rejection are three: 1) explicit rejection (HTTP code "
            "403), 2) rejection by the local FaaS platform (HTTP code 5xx), "
            "3) forwarding to another DFaaS node followed by rejection (HTTP "
            "code 5xx)."
        )
    )

    parser.add_argument("--input", required=True, type=Path, help="Path to input CSV")
    parser.add_argument("--output", required=True, type=Path, help="Path to output PDF")
    parser.add_argument(
        "--iter-duration",
        type=int,
        default=60,
        help="Iteration duration in seconds (unused but kept for compatibility)",
    )
    args = parser.parse_args()

    df = pd.read_csv(args.input)

    node_name = args.input.parent.name

    fig = plot_rejection_breakdown(df, node_name)
    fig.savefig(args.output, bbox_inches="tight")
    print(f"Saved plot to {args.output.as_posix()}")


if __name__ == "__main__":
    main()
