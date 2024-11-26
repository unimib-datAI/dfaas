#!/usr/bin/env python3

import numpy as np
import pandas as pd
import sys
import os
import matplotlib.pyplot as plt

# Dictionary with nodes' names and corresponding IP addresses
nodes_dict = {}

# Necessary for plots of parallel attacks (merged results), specifies the maximum rate on a single node
max_load_on_node = 600

# Necessary for plots of parallel attacks (merged results), indicates the different phases of test, with duration and total rate of each phase
sectors_dict = {
    '0': {'duration': '300',
          'rate': '350'},
    '1': {'duration': '300',
          'rate': '550'},
    '2': {'duration': '300',
          'rate': '1050'},
}

'''
    Read json file containing vegeta export.
    
    This function takes only one param: the path of json file.
    This function returns a dataframe containing status codes, timestamps, X-Server and URL headers values.
    The returned dataframe represent one packet send for each row with respective response.
'''

def read_json(path):
    code_stream = os.popen('cat ' + path + ' | jq -r .code')
    url_stream = os.popen('cat ' + path + ' | jq -r .url')
    timestamp_stream = os.popen('cat ' + path + ' | jq -r .timestamp')
    x_server_ok_stream = os.popen('cat ' + path + ' | jq -r \'select(.code == 200) | .headers."X-Server"[]\'')
    timestamp_ok_stream = os.popen('cat ' + path + ' | jq -r \'select(.code == 200) | .timestamp\'')

    codes = code_stream.read().splitlines()
    timestamp = timestamp_stream.read().splitlines()
    x_server_ok = x_server_ok_stream.read().splitlines()
    urls = url_stream.read().splitlines()
    timestamp_ok = timestamp_ok_stream.read().splitlines()
    
    df = pd.DataFrame.from_dict(
        {"timestamp": pd.to_datetime(timestamp), "code": codes, "url": urls})
    df_x_server_ok = pd.DataFrame.from_dict(
        {"timestamp": pd.to_datetime(timestamp_ok), "X-Server": x_server_ok})

    df = df.join(df_x_server_ok.set_index('timestamp'), on='timestamp')
    df.sort_values("timestamp", inplace=True)
    df.reset_index(drop=True, inplace=True)

    return df


'''
    Compute one column each parameter. Each of this columns has 1 correpsonding to rows that returns this specific value, 0 otherwise.
    "node_x_server" columns specify if the node has replied, "node_url_ok" ("node_url_err") columns specify if the request directed to the node has been
    (hasn't been) succesfully processed, "node_forwarded_req" columns specifiy if the request directed to the node has been forwarded to another node
    and then successfully processed.
'''

def compute_df_columns(df):
    df["Success"] = np.where(df["code"] == '200', 1, 0)

    df["Error"] = np.where(df["code"] != '200', 1, 0)
    
    for node in nodes_dict:
        col_x_server = node + "_x_server"
        df[col_x_server] = np.where(df["X-Server"] == nodes_dict[node], 1, 0)
        col_url_ok = node + "_url_ok"
        col_url_err = node + "_url_err"
        df[col_url_ok] = np.where((df["url"].str.contains(nodes_dict[node])) & (df["code"] == '200'), 1, 0)
        df[col_url_err] = np.where((df["url"].str.contains(nodes_dict[node])) & (df["code"] != '200'), 1, 0)
        col_forwarded_req = node + "_forwarded_req"
        df[col_forwarded_req] = np.where((df["url"].str.contains(nodes_dict[node])) & (df["code"] == '200') & (df["X-Server"] != nodes_dict[node]), 1, 0)

    return df


'''
    Group row by 'rate', obtaining one row for each second of test.   
    This function takes as input the source dataset, the rate of the test attack, if the input file is a "merged results" file, and return grouped dataset.
'''

def group_by_resp_for_seconds(df, rate, is_merged):
    url_ok_cols = []
    url_err_cols = []
    x_server_cols = []
    forwarded_req_cols = []
    df_sectors = []

    for node in nodes_dict:
        x_server_cols.append(node + "_x_server")
        url_ok_cols.append(node + "_url_ok")
        url_err_cols.append(node + "_url_err")
        forwarded_req_cols.append(node + "_forwarded_req")
    
    if not is_merged:
        df_by_second = df.groupby(lambda x: x // rate)[['Success', 'Error'] + x_server_cols + url_ok_cols + url_err_cols + forwarded_req_cols].sum()
    else:
        start_index = 0
        for sector in sectors_dict:
            rate = int(sectors_dict[sector]['rate'])
            rows_num = int(sectors_dict[sector]['duration']) * rate
            end_index = start_index + rows_num
            df_sectors.append(df[start_index:end_index].groupby(lambda x: x // rate)[['Success', 'Error'] + x_server_cols + url_ok_cols + url_err_cols + forwarded_req_cols].sum())
            start_index = end_index
        df_by_second = pd.concat(df_sectors, ignore_index=True)
        
    return df_by_second


'''
    Compute nodes' responses subplots.
'''

def compute_nodes_subplots(df, cols, labels, path, max_y, width, height):
    colors = plt.rcParams["axes.prop_cycle"]()
    f, axes = plt.subplots(len(cols), 1, sharey=True)
    f.set_size_inches(width, height)
    f.supxlabel('Time (s)')
    f.supylabel('Response rate [resp/s]')
    for i in range(len(cols)):
        c = next(colors)["color"]
        axes[i].set_ylim(0, max_y)
        axes[i].plot(df.index, df[cols[i]], color=c, label=labels[i])
        axes[i].grid()
    f.legend(loc='upper right')
    plt.savefig(path)


'''
    Compute nodes' responses and cumulatives subplots.
'''

def compute_nodes_subplots_and_cumulatives(df, cols, labels, path, max_y, width, height, log=False):
    colors = plt.rcParams["axes.prop_cycle"]()
    f, axes = plt.subplots(len(cols) - 1, 1)
    f.set_size_inches(width, height)
    f.supxlabel('Time (s)')
    f.supylabel('Response rate [resp/s]')
    for i in range(len(cols) - 2):
        c = next(colors)["color"]
        axes[i].set_ylim(0, max_y)
        axes[i].plot(df.index, df[cols[i]], color=c, label=labels[i])
        axes[i].grid()
    f.legend(loc='upper right')
    c = next(colors)["color"]
    if log:
        axes[len(cols) - 2].set_yscale('log')
    axes[len(cols) - 2].set_ylim(0, 100000)
    axes[len(cols) - 2].plot(df.index, np.cumsum(df[cols[len(cols) - 2]]), color='purple', label='Success')
    axes[len(cols) - 2].plot(df.index, np.cumsum(df[cols[len(cols) - 1]]), color='red', label='Error')
    axes[len(cols) - 2].set(ylabel='Cumulatives')
    axes[len(cols) - 2].legend(loc='upper right')
    axes[len(cols) - 2].grid()
    plt.savefig(path)


'''
    Compute nodes' client side success rate subplots with forwarded requests.
'''

def compute_nodes_subplots_with_forwarded_req(df, cols, labels, path, max_y, width, height):
    colors = plt.rcParams["axes.prop_cycle"]()
    f, axes = plt.subplots(int(len(cols) / 2), 1, sharey=True)
    f.set_size_inches(width, height)
    f.supxlabel('Time (s)')
    f.supylabel('Client side success rate [req/s]')
    for i in range(int(len(cols) / 2)):
        c = next(colors)["color"]
        axes[i].set_ylim(0, max_y)
        axes[i].plot(df.index, df[cols[i]], color=c, label=labels[i])
        axes[i].plot(df.index, df[cols[i + int(len(cols) / 2)]], color='gray', linestyle='dotted')
        axes[i].grid()
    f.legend(loc='upper right')
    plt.savefig(path)


'''
    Compute nodes' cumulatives subplots.
'''

def compute_nodes_cumulatives_subplots(df, cols_succ, cols_err, path, width, height, log=False):
    colors = plt.rcParams["axes.prop_cycle"]()
    f, axes = plt.subplots(len(cols_succ), 1, sharey=True)
    f.set_size_inches(width, height)
    f.supxlabel('Time (s)')
    f.supylabel('Cumulatives')
    for i in range(len(cols_succ)):
        c = next(colors)["color"]
        if log:
            axes[i].set_yscale('log')
        axes[i].title.set_text(list(nodes_dict.keys())[i])
        axes[i].plot(df.index, np.cumsum(df[cols_succ[i]]), color=c, label='Success')
        axes[i].plot(df.index, np.cumsum(df[cols_err[i]]), color='red', label='Error')
        axes[i].legend(loc='upper right')
        axes[i].grid()
    plt.savefig(path)


'''
    Compute cumulatives plot.
'''

def compute_cumulatives_plot(df, cols, labels, path, width, height, log=False):
    plt.figure(figsize=(width, height))
    plt.xlabel("Time (s)")
    plt.ylabel("Cumulatives")
    if log:
        plt.yscale('log')
    for i in range(len(cols)):
        plt.plot(df.index, np.cumsum(df[cols[i]]), label=labels[i])
    plt.legend(loc="upper right")
    plt.grid()
    plt.savefig(path)


'''
    Compute report with success rate values.

'''

def compute_report(input_file_path, output_dir_path, is_merged):
    succ_responses_num = int(os.popen('cat ' + input_file_path + '| jq -r \'select(.code == 200) | .code\' | jq -s \'. | length\'').read())
    total_responses = int(os.popen('cat ' + input_file_path + '| jq -r \'.code\' | jq -s \'. | length\'').read())
    success_rate = ( succ_responses_num / total_responses ) * 100
    if is_merged:
        nodes_succ_rate = {}
        for node in nodes_dict:
            succ_resp_num = int(os.popen('cat ' + input_file_path + '| jq -r \'select((.code == 200) and ( .url | contains(\"' + nodes_dict[node] + '\"))) | .code\' | jq -s \'. | length\'').read())
            tot_req = int(os.popen('cat ' + input_file_path + '| jq -r \'select(.url | contains(\"' + nodes_dict[node] + '\")) | .code\' | jq -s \'. | length\'').read())
            if tot_req > 0:
                nodes_succ_rate[node] = ( succ_resp_num / tot_req ) * 100
    with open(output_dir_path + "/report_values.txt", "w") as text_file:
        text_file.write(f'Success rate: {success_rate:.2f}%\n')
        if is_merged:
            for node in nodes_succ_rate:
                text_file.write(f'Success rate {node}: {nodes_succ_rate[node]:.2f}%\n')


'''
    The script takes as input the following parameters: 'results.json' file path, output directory, rate of the attack
    and a boolean parameter that specifies if the input file has been obtained merging results files of multiple parallel attacks.
    If merged is true, "Rate" parameter isn't used and can be set to any value.
'''

def main(argv):
    file_input, output_dir, rate, merged = str(argv[0]), str(argv[1]), int(argv[2]), eval(argv[3])

    # Reading datasets from json file.
    df1 = read_json(file_input)

    # Calcolate additional columns.
    df1 = compute_df_columns(df1)

    # Group rows by 'rate' --> one for each second.
    df_by_second1 = group_by_resp_for_seconds(df1, rate, merged)
    
    # Compute plots
    if not merged:
        cols = []
        for node in nodes_dict:
            cols.append(node + "_x_server")
        compute_nodes_subplots_and_cumulatives(df_by_second1, cols + ['Success', 'Error'], list(nodes_dict.keys()) + ['Success', 'Error'], output_dir + '/plot-nodes-responses-and-cumulatives.png', rate + 50, 10, 15, log=True)
        compute_nodes_subplots(df_by_second1, cols, list(nodes_dict.keys()), output_dir + '/plot-nodes-responses.png', rate + 50, 10, 10)
        compute_cumulatives_plot(df_by_second1, ['Success', 'Error'], ['Success', 'Error'], output_dir + '/plot-cumulatives.png', 10, 5)
        compute_cumulatives_plot(df_by_second1, ['Success', 'Error'], ['Success', 'Error'], output_dir + '/plot-cumulatives-log.png', 10, 5, log=True)
    else:
        cols_succ = []
        cols_err = []
        cols_forwarded = []
        cols = []
        for node in nodes_dict:
            cols_succ.append(node + "_url_ok")
            cols_err.append(node + "_url_err")
            cols_forwarded.append(node + "_forwarded_req")
            cols.append(node + "_x_server")
            compute_cumulatives_plot(df_by_second1, [node + "_url_ok", node + "_url_err"], ['Success', 'Error'], output_dir + f'/plot-cumulatives-log-{node}.png', 10, 5, log=True)
        compute_nodes_subplots_with_forwarded_req(df_by_second1, cols_succ + cols_forwarded, list(nodes_dict.keys()), output_dir + '/plot-succ-by-node-with-forwarded.png', max_load_on_node + 20, 10, 10)
        compute_nodes_subplots(df_by_second1, cols_succ, list(nodes_dict.keys()), output_dir + '/plot-succ-by-node.png', max_load_on_node + 20, 10, 10)
        compute_nodes_subplots(df_by_second1, cols_err, list(nodes_dict.keys()), output_dir + '/plot-err-by-node.png', max_load_on_node + 20, 10, 10)
        compute_nodes_cumulatives_subplots(df_by_second1, cols_succ, cols_err, output_dir + '/plot-cumulatives-by-node.png', 10, 10)
        compute_nodes_cumulatives_subplots(df_by_second1, cols_succ, cols_err, output_dir + '/plot-cumulatives-by-node-log.png', 10, 10, log=True)            
        compute_nodes_subplots(df_by_second1, cols, list(nodes_dict.keys()), output_dir + '/plot-nodes-responses.png', max_load_on_node + 20, 10, 10)
        compute_nodes_subplots_and_cumulatives(df_by_second1, cols + ['Success', 'Error'], list(nodes_dict.keys()) + ['Success', 'Error'], output_dir + '/plot-nodes-responses-and-cumulatives.png', max_load_on_node + 20, 10, 15, log=True)

    # Compute report with success rates
    compute_report(file_input, output_dir, is_merged=merged)


if __name__ == '__main__':
    raw_nodes = os.getenv("NODES")
    if raw_nodes is None:
        print("Missing 'NODES' env variable, exiting...")
        exit(1)
    nodes = raw_nodes.split(":")
    if len(nodes) != 3:
        print("Only 3 nodes are supported, exiting...")
        exit(1)
    nodes_dict["Node light"] = nodes[0]
    nodes_dict["Node mid"] = nodes[1]
    nodes_dict["Node heavy"] = nodes[2]

    main(sys.argv[1:])
