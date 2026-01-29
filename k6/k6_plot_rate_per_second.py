import json

import pandas as pd
import matplotlib.pyplot as plt
import numpy as np

# Load k6 CSV metrics.
df = pd.read_csv("k6_results.csv.gz")
http_reqs = df[df["metric_name"] == "http_reqs"]
rate_per_second = http_reqs.groupby("timestamp")["metric_value"].sum()
start_ts = rate_per_second.index.min()
seconds_since_start = rate_per_second.index - start_ts

# Load and expand input trace.
with open("input_requests_traces.json") as f:
    trace_data = json.load(f)

trace_rates = trace_data["0"]["0"][
    :10
]  # Take the first 10 values as reference rates, each representing one minute.

# Build expanded (per-second) reference trace: each value repeats for 60 seconds.
trace_expanded = np.repeat(trace_rates, 60)

# Determine total seconds from the k6 measurement.
max_time = seconds_since_start.max() + 1

# Align the length of the reference trace to the measured data.
expanded_len = max(len(trace_expanded), max_time)
trace_expanded_full = np.zeros(expanded_len)
trace_expanded_full[: len(trace_expanded)] = trace_expanded[:expanded_len]

# Set up the x-axis as seconds since start.
x = np.arange(expanded_len)

# Prepare the plot with increased width.
fig, ax = plt.subplots(figsize=(16, 5))

# Plot measured (k6) rates as a blue line.
ax.plot(
    seconds_since_start,
    rate_per_second.values,
    linestyle="-",
    color="blue",
    label="Measured (k6)",
    zorder=3,
)

# Plot expanded reference rates as a red line.
ax.plot(
    x[:max_time],
    trace_expanded_full[:max_time],
    linestyle="-",
    color="red",
    label="Input trace (reference, 1-min steps)",
    zorder=2,
)

# Set axis labels and title.
ax.set_xlabel("Seconds since start")
ax.set_ylabel("Requests per Second")
ax.set_title("k6 Requests per Second vs Input Trace")

# Set y-axis minimum to zero.
ax.set_ylim(bottom=0)

# Enable both primary and secondary grid lines in the background.
ax.grid(which="both", linestyle="-", linewidth=0.5, alpha=0.7, zorder=1)
ax.minorticks_on()
ax.set_axisbelow(True)

# Display legend.
ax.legend()

plt.tight_layout()
plt.savefig("k6_rate_per_second.pdf")

print("Plot saved as k6_rate_per_second.pdf.")
