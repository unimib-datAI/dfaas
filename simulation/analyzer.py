import pandas as pd
import numpy as np
import matplotlib.pyplot as plt

base_dir = "test/reports/"
function_names = ["funca", "qrcode", "ocr"]
algorithms_to_compare = ["base_strategy", "random_strategy", "empirical_strategy"]
simulation_minutes = 7
rates_for_algo = {}

def calculate_rates(table, func, max_rates, invoc_rates):
    incoming_requests_for_node = table.sum(axis=0)

    success_rate = 0
    for node in table.columns:
        if incoming_requests_for_node[node] > max_rates[node]:
            success_rate += max_rates[node]
        else:
            success_rate += incoming_requests_for_node[node]

    tot_invoc_rate = invoc_rates.sum(axis=0)
    reject_num = tot_invoc_rate - success_rate

    print("====> Success req. ({}) + Rejected req. ({}) == {}: {}".format(
            success_rate,
            reject_num,
            tot_invoc_rate,
            success_rate + reject_num == tot_invoc_rate
        )
    )

    success_rate = (success_rate / tot_invoc_rate) if tot_invoc_rate > 0 and success_rate <= tot_invoc_rate else 1.0
    reject_rate = 1.0 - success_rate

    print("Success rate for func {} is {}".format(func, success_rate))
    print("Reject rate for func {} is {}".format(func, reject_rate))
    
    print("====> SR + RR == 1: {}".format(success_rate+reject_rate == 1))
    
    # Reject num is multiplied by 60 that are seconds between each agent execution
    # Note: This is based on the assumption that the traffic will be more or less 
    # constant during this minute
    return success_rate, reject_rate, reject_num*60

def export_for_minute_rates(func, rates):
    # Plot configurations
    plt.figure(figsize=(20, 10))
    plt.title("Success rate for function {} during 6 minutes of experiment".format(func))
    plt.xlabel("Minute")
    plt.ylabel("Success rate")

    df = pd.DataFrame(data=rates, index=[i for i in range(0, 7)])
    print(df)

    for column in df.columns:
        plt.plot(df.index, df[column], label="Success rate for {}".format(column))

    # Plot configurations
    plt.legend(loc="lower left")
    plt.grid()

    plt.savefig("comparison_{}.png".format(func))

# Used only for initialization
for func in function_names:
    rates_for_algo[func] = {}

for algo in algorithms_to_compare:
    funca_sr, funca_rr, funca_reject_num = [], [], []
    qrcode_sr, qrcode_rr, qrcode_reject_num = [], [], []
    ocr_sr, ocr_rr, ocr_reject_num = [], [], []

    print("-------------------------- ALGO {} --------------------------".format(algo))
    base_path = base_dir + algo + "/"
    for minute in range(0, simulation_minutes):
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

        sr, rr, rn = calculate_rates(df_funca, "funca", df_max_rate["funca"], df_invoc_rate["funca"])
        funca_sr.append(sr)
        funca_rr.append(rr)
        funca_reject_num.append(rn)
        rates_for_algo["funca"][algo] = funca_sr

        sr, rr, rn = calculate_rates(df_qrcode, "qrcode", df_max_rate["qrcode"], df_invoc_rate["qrcode"])
        qrcode_sr.append(sr)
        qrcode_rr.append(rr)
        qrcode_reject_num.append(rn)
        rates_for_algo["qrcode"][algo] = qrcode_sr

        sr, rr, rn = calculate_rates(df_ocr, "ocr", df_max_rate["ocr"], df_invoc_rate["ocr"])
        ocr_sr.append(sr)
        ocr_rr.append(rr)
        ocr_reject_num.append(rn)
        rates_for_algo["ocr"][algo] = ocr_sr

        print("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")
        
    print("STATS FOR ALGO {}".format(algo))
    
    # Utility print for success/reject rate and reject nume for func
    # print(" > Mean success rate for funca: {}".format(np.mean(funca_sr)))
    # print(" > Mean reject rate for funca: {}".format(np.mean(funca_rr)))
    # print(" > Rejected requests for funca: {}".format(np.sum(funca_reject_num)))

    # print(" > Mean success rate for qrcode: {}".format(np.mean(qrcode_sr)))
    # print(" > Mean reject rate for qrcode: {}".format(np.mean(qrcode_rr)))
    # print(" > Rejected requests for qrcode: {}".format(np.sum(qrcode_reject_num)))

    # print(" > Mean success rate for ocr: {}".format(np.mean(ocr_sr)))
    # print(" > Mean reject rate for ocr: {}".format(np.mean(ocr_rr)))
    # print(" > Rejected requests for ocr: {}".format(np.sum(ocr_reject_num)))
    
    print("     > Mean success rate: {:0.2f}%".format(
        np.mean([np.mean(x) for x in [funca_sr, qrcode_sr, ocr_sr]]) * 100
    ))
    print("         > Mean success rate in stress period: {:0.2f}%".format(
        np.mean([np.mean(x) for x in [funca_sr[1:6], qrcode_sr[1:6], ocr_sr[1:6]]]) * 100
    ))
    print("     > Total rejected requests: {} req".format(
        np.sum([np.sum(x) for x in [funca_reject_num, qrcode_reject_num, ocr_reject_num]])
    ))
    print("----------------------------------------------------------------------------")

# Export print for comparison
export_for_minute_rates("funca", rates_for_algo["funca"])
export_for_minute_rates("qrcode", rates_for_algo["qrcode"])
export_for_minute_rates("ocr", rates_for_algo["ocr"])
