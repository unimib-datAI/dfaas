import pandas as pd
import numpy as np

base_dir = "test/reports/"
function_names = ["funca", "qrcode", "ocr"]

def calculate_rates(table, func, max_rates, invoc_rates):
    incoming_requests_for_node = table.sum(axis=0)
    #print(incoming_requests_for_node)
    #print(invoc_rates)
    #print(max_rates)

    success_rate = 0
    #reject_rate = 0
    for node in table.columns:
        if incoming_requests_for_node[node] > max_rates[node]:
            success_rate += max_rates[node]
            #reject_rate += incoming_requests_for_node[node] - max_rates[node]
        else:
            success_rate += incoming_requests_for_node[node]

    tot_invoc_rate = invoc_rates.sum(axis=0)
    #print(tot_invoc_rate)
    success_rate = (success_rate / tot_invoc_rate) if tot_invoc_rate > 0 and success_rate <= tot_invoc_rate else 1.0
    #reject_rate = (reject_rate / tot_invoc_rate) if tot_invoc_rate > 0 else 0.0
    reject_rate = 1.0 - success_rate

    print("Success rate for func {} is {}".format(func, success_rate))
    print("Reject rate for func {} is {}".format(func, reject_rate))
    
    print("====> SR + RR == 1: {}".format(success_rate+reject_rate == 1))
    
    return success_rate, reject_rate


for minute in range(0, 7):
    print("MINUTE {}".format(minute))
    print(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
    path = base_dir + "minute_" + str(minute) + "/"
    
    # Load dataframe from foulders
    df_funca = pd.read_csv(path + "funca.csv", delimiter='\t', header=0, index_col=0)

    print("================ FORWARDED REQUESTS FUNCA ================")
    print(df_funca)
    print("==========================================================")

    df_qrcode = pd.read_csv(path + "qrcode.csv", delimiter='\t', header=0, index_col=0)

    print("================ FORWARDED REQUESTS QRCODE ================")
    print(df_qrcode)
    print("===========================================================")
    
    df_ocr = pd.read_csv(path + "ocr.csv", delimiter='\t', header=0, index_col=0)

    print("================ FORWARDED REQUESTS OCR ================")
    print(df_ocr)
    print("========================================================")

    df_invoc_rate = pd.read_csv(path + "invoc_rates.csv", delimiter='\t', header=0, index_col=0)
    print("================ INVOCATION RATES ==================")
    print(df_invoc_rate)
    print("====================================================")

    df_max_rate = pd.read_csv(path + "max_rates.csv", delimiter='\t', header=0, index_col=0)
    print("================ MAX RATES =========================")
    print(df_max_rate)
    print("====================================================")

    calculate_rates(df_funca, "funca", df_max_rate["funca"], df_invoc_rate["funca"])
    calculate_rates(df_qrcode, "qrcode", df_max_rate["qrcode"], df_invoc_rate["qrcode"])
    calculate_rates(df_ocr, "ocr", df_max_rate["ocr"], df_invoc_rate["ocr"])
    
    print("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")
