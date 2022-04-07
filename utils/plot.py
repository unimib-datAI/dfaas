#!/usr/bin/env python3

import numpy as np
import pandas as pd
import sys
import json
import os
import matplotlib.pyplot as plt
import matplotlib.dates as mdates

status_code_map = {
    '261': '261 - Node 1 response',
    '262': '262 - Node 2 response',
    '263': '263 - Node 3 response',
    '403': '403 - Forbidden',
    '429': '429 - Too Many Requests',
    '503': '503 - Service Unavailable',
}

'''
    Read json file containing vegeta export.
    
    This function takes only one param: the path of json file.
    This function returns a dataframe containing status codes and timestamps.
    The returned dataframe represent one packet send for each row with 
    respective response.
'''
def read_json(path):
    code_stream = os.popen('cat ' + path + ' | jq -r .code')
    codes = code_stream.read().splitlines()
    timestamp_stream = os.popen('cat ' + path + ' | jq -r .timestamp')
    timestamp = timestamp_stream.read().splitlines()
    df = pd.DataFrame.from_dict(
        {"timestamp": pd.to_datetime(timestamp), "code": codes})
    return df

'''
    Compute one column for each status code in dataframe passaed as param.
    
    Each of this columns has 1 correpsonding to rows that returns this specific state,
    0 otherwise.
'''
def compute_df_columns(df):
    for status_code in df["code"].unique():
        df[status_code] = np.where(df["code"] == status_code, 1, 0)
    #print((df.sum(axis=1) == 1).all())
    return df

'''
    Group row by 'rate', obtaining one row for each minute of test.
    
    Columns mantained are one for each status code, with number of times that 
    the code occurs in this minute (among 'rate' responses).
    This function also add to new dataset a column containing timestamp for each second
    of testing. This column is used as index in new dataset.
    
    This function takes as input the source dataset and return grouped dataset.
'''
def group_by_resp_for_minutes(df, rate):
    df_by_second = df.groupby(lambda x: x // rate)[df["code"].unique()].sum()
    #print((df_by_second.sum(axis=1) == 50).all())
    
    # Attach for each second corresponding timestamp.
    column = [df["timestamp"][i] for i in range(0, len(df), rate)]
    #print(len(column))
    
    df_by_second["start timestamp"] =  column
    df_by_second.set_index("start timestamp", inplace=True)
    
    #print(len(column) == len(df_by_second))
    
    return df_by_second

'''
    Setup plot congiguration.
'''
def setup_plot():
    plt.figure(figsize=(20, 10))
    plt.xlabel("Time - Format [Hour:Minutes]")
    plt.ylabel("Respenses [rate resp/s]")
    
'''
    Computer chart for each row in the dataset, passed as param.
'''
def compute_chart(df):
    for col in sorted(df.columns):
        plt.plot(df.index, df[col], label=status_code_map[col])

'''
    Export plot to png.
'''
def export_plot(path):
    dtFmt = mdates.DateFormatter('%H:%M')  # define the formatting
    plt.gca().xaxis.set_major_formatter(dtFmt)
    
    plt.legend(loc="upper right")
    plt.grid()

    #plt.show()
    plt.savefig(path)

def main(argv):
    file_input, file_output, rate = str(argv[0]), str(argv[1]), int(argv[2])
    
    # Reading datasets from json files.
    df1 = read_json(file_input)
    #df2 = read_json('malicius-testing/2021-04-15-110614/results.json')
    
    # Calcolate additional columns (one for each error code.).
    df1 = compute_df_columns(df1)
    #df2 = compute_df_columns(df2)
    
    # Group rows by 'rate' --> one for each second.
    df_by_second1 = group_by_resp_for_minutes(df1, rate)
    #df_by_second2 = group_by_resp_for_minutes(df2)
    
    # Debugging.
    #print("######### DF1 #########")
    #print(df1.head(52))
    #print(df_by_second1)
    #print(df_by_second1.columns)
    #print()
    #print("######### DF2 #########")
    #print(df_by_second2)
    #print(df_by_second2.columns)
    
    # Set fig size and axis label.
    setup_plot()
    
    # Draw chart.
    compute_chart(df_by_second1)
    #compute_chart(df_by_second2)
    
    # Export on png file.
    export_plot(file_output)
    
if __name__ == '__main__':
    main(sys.argv[1:])
