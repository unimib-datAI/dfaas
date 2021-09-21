import pandas as pd
import numpy as np

base_dir = "test/reports/"
function_names = ["funca", "qrcode", "ocr"]
algorithms_to_compare = ["base_strategy", "random_strategy", "empirical_strategy"]

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

for algo in algorithms_to_compare:
    funca_sr, funca_rr = [], []
    qrcode_sr, qrcode_rr = [], []
    ocr_sr, ocr_rr = [], []

    print("-------------------------- ALGO {} --------------------------".format(algo))
    base_path = base_dir + algo + "/"
    for minute in range(0, 7):
        print("MINUTE {}".format(minute))
        print(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")
        path = base_path + "minute_" + str(minute) + "/"
        
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

        sr, rr = calculate_rates(df_funca, "funca", df_max_rate["funca"], df_invoc_rate["funca"])
        funca_sr.append(sr)
        funca_rr.append(rr)

        sr, rr = calculate_rates(df_qrcode, "qrcode", df_max_rate["qrcode"], df_invoc_rate["qrcode"])
        qrcode_sr.append(sr)
        qrcode_rr.append(rr)

        sr, rr = calculate_rates(df_ocr, "ocr", df_max_rate["ocr"], df_invoc_rate["ocr"])
        ocr_sr.append(sr)
        ocr_rr.append(rr)

        print("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")
        
    print("STATS FOR ALGO {}".format(algo))
    print(" > Mean success rate for funca: {}".format(np.mean(funca_sr)))
    print(" > Mean reject rate for funca: {}".format(np.mean(funca_rr)))

    print(" > Mean success rate for qrcode: {}".format(np.mean(qrcode_sr)))
    print(" > Mean reject rate for qrcode: {}".format(np.mean(qrcode_rr)))

    print(" > Mean success rate for ocr: {}".format(np.mean(ocr_sr)))
    print(" > Mean reject rate for ocr: {}".format(np.mean(ocr_rr)))
    print("----------------------------------------------------------------------------")
