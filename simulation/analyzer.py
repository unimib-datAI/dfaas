import pandas as pd
import numpy as np
import matplotlib.pyplot as plt
from config_manager import ConfigManager

config_manager = ConfigManager()

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
    #print(df)

    for column in df.columns:
        plt.plot(df.index, df[column], label="Success rate for {}".format(column))

    # Plot configurations
    plt.legend(loc="lower left")
    plt.grid()

    plt.savefig(config_manager.ANALYZER_OUTPUT_PATH + 
                "comparison_{}.png".format(func))


def export_index_comparison_table(df):
    df.to_csv(config_manager.ANALYZER_OUTPUT_PATH +
              "index_comparison.csv", sep='\t', encoding='utf-8')


def main():
    rates_for_algo = {}
    index_comparison = pd.DataFrame(index=config_manager.INDEX_TO_COMPARE)

    # Used only for initialization
    for func in config_manager.FUNCTION_NAMES:
        rates_for_algo[func] = {}

    for algo in config_manager.STRATEGIES:
        x_func_success_rate = {}
        x_func_reject_rate = {}
        x_func_reject_num = {}
        
        for func in config_manager.FUNCTION_NAMES:
            x_func_success_rate[func] = []
            x_func_reject_rate[func] = []
            x_func_reject_num[func] = []

        print("-------------------------- ALGO {} --------------------------".format(algo))

        # Create path for recover tables        
        base_path = config_manager.SIMULATION_TABLES_OUTPUT_PATH + algo + "/"

        for minute in range(0, config_manager.SIMULATION_MINUTES):
            print("MINUTE {}".format(minute))
            print(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>")

            # Complete path for load tables
            path = base_path + "minute_" + str(minute) + "/"

            # For each minute load invocaion_rate and max_rate table
            df_invoc_rate = pd.read_csv(path + "invoc_rates.csv", delimiter='\t', header=0, index_col=0)
            print("================ INVOCATION RATES ==================")
            print(df_invoc_rate)
            print("====================================================")

            df_max_rate = pd.read_csv(path + "max_rates.csv", delimiter='\t', header=0, index_col=0)
            print("================ MAX RATES =========================")
            print(df_max_rate)
            print("====================================================")

            # For each minute and foreach function load datafram
            for func in config_manager.FUNCTION_NAMES:
                df = pd.read_csv(path + func + ".csv", delimiter='\t', header=0, index_col=0)

                print("================ FORWARDED REQUESTS for {} ================".format(func))
                print(df)
                print("==========================================================")

                sr, rr, rn = calculate_rates(df, func, df_max_rate[func], df_invoc_rate[func])
                x_func_success_rate[func].append(sr)
                x_func_reject_rate[func].append(rr)
                x_func_reject_num[func].append(rn)
                rates_for_algo[func][algo] = x_func_success_rate[func]

            print("<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")

        print("STATS FOR ALGO {}".format(algo))

        # Utility print for success/reject rate and reject nume for func
        # TODO: fix it to work with new dictionaties
        #
        # print(" > Mean success rate for funca: {}".format(np.mean(funca_sr)))
        # print(" > Mean reject rate for funca: {}".format(np.mean(funca_rr)))
        # print(" > Rejected requests for funca: {}".format(np.sum(funca_reject_num)))

        # print(" > Mean success rate for qrcode: {}".format(np.mean(qrcode_sr)))
        # print(" > Mean reject rate for qrcode: {}".format(np.mean(qrcode_rr)))
        # print(" > Rejected requests for qrcode: {}".format(np.sum(qrcode_reject_num)))

        # print(" > Mean success rate for ocr: {}".format(np.mean(ocr_sr)))
        # print(" > Mean reject rate for ocr: {}".format(np.mean(ocr_rr)))
        # print(" > Rejected requests for ocr: {}".format(np.sum(ocr_reject_num)))

        # Other utility prints
        mean_success_rate = np.mean([np.mean(srates) for k, srates in x_func_success_rate.items()]) * 100
        print("     > Mean success rate: {:0.2f}%".format(
            mean_success_rate
        ))

        # Mean success rate calculated during high traffic preiod (minutes from 1 to 5)
        mean_success_rate_stress_period = np.mean(
            [np.mean(srates[1:6]) for k, srates in x_func_success_rate.items()]) * 100
        print("         > Mean success rate during stress period (from minute 1 to 5): {:0.2f}%".format(
            mean_success_rate_stress_period
        ))

        # Total rejected requests num calculated for each algorithm across minutes
        total_reject_requests = np.sum(
            [np.sum(rejnums) for k, rejnums in x_func_reject_num.items()])
        print("     > Total rejected requests: {} req".format(
            total_reject_requests
        ))
        print(
            "----------------------------------------------------------------------------")

        index_comparison[algo] = [
            mean_success_rate,
            mean_success_rate_stress_period,
            total_reject_requests
        ]

    # Export print for comparison
    for func in config_manager.FUNCTION_NAMES:
        export_for_minute_rates(func, rates_for_algo[func])

    # Export index comparison table
    print("> INDEX COMPARISON TABLE")
    print(index_comparison.T)
    export_index_comparison_table(index_comparison.T)


# Call main program.
if __name__ == "__main__":
    main()
