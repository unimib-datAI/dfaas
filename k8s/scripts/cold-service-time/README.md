# Cold service time estimator

This is a small Python script (`main.py`) designed to measure the cold service
time of OpenFaaS functions deployed on a Kubernetes cluster. For a specified
function, it performs repeated tests and writes each result as a row in a CSV
file.

The available columns are:

* `timestamp`: the timestamp of the test,
* `function_name`: name of the function,
* `cold_start_service_time_s`: total seconds from the start of the request until
  the last byte of the response is received,
* `time_to_first_byte_s`: seconds from the start of the request until the client
  receives the first response byte (TTFB),
* `service_time_s`: reported execution time from OpenFaaS (excluding proxy and
  client transfer time, based on the `x-duration-seconds` header).

## How to run

You only need Python 3, no additional third-party Python packages are required.
Ensure that `curl`, `sudo`, and `kubectl` are installed and properly configured,
especially `kubectl` with access to your cluster. The script assumes the target
OpenFaaS function is already deployed and available on the Kubernetes cluster.

You can customize the number of tests using `--cycles` (default is 10), and set
the output CSV path using `--output` (default is
`cold_service_time_FUNCTION.csv`).

Example usage:

```console
$ python3 main.py 10.0.2.38:30080 figlet
```

The script will measure the cold start service time of the `figlet` function
deployed at `http://10.0.2.38:30080/function/figlet` for 10 cycles, and save the
output to `cold_service_time_figlet.csv`.

Example output:

```console
$ head cold_service_time_figlet.csv
timestamp,function_name,cold_start_service_time_s,time_to_first_byte_s,service_time_s
2026-02-03T17:01:10.755862,figlet,2.747331,2.747274,0.002289
2026-02-03T17:01:14.821116,figlet,3.559473,3.559412,0.002586
2026-02-03T17:01:18.796313,figlet,2.950073,2.950001,0.002703
2026-02-03T17:01:23.825908,figlet,4.474774,4.474707,0.002255
2026-02-03T17:01:27.847611,figlet,2.749221,2.749164,0.002298
2026-02-03T17:01:31.802133,figlet,2.650962,2.650899,0.003473
2026-02-03T17:01:36.867280,figlet,4.266747,4.266664,0.002583
2026-02-03T17:01:41.865855,figlet,4.275085,4.275030,0.002299
2026-02-03T17:01:47.949893,figlet,4.777964,4.777899,0.002618
```

The script includes a list of supported functions (like `figlet`), each with its
corresponding request body. If you need to test additional functions, you will
need to manually update this list with the appropriate function names and
request payloads.

## How it works

OpenFaaS Community Edition (CE) does not support scale-to-zero (reserved for
Pro), so functions are not automatically scaled down when idle. To simulate and
measure a cold start (the latency experienced when invoking a function after a
period of inactivity that requires a new pod to start up) the script deletes all
pods for the chosen function using `kubectl`. As Kubernetes begins to launch a
new pod to replace them, the script waits for the existing pods to enter the
"Terminating" state (which ensures no new requests are routed to the old pods),
then issues a request using `curl` and measures the relevant latencies. This
process is repeated for each test cycle.

There are inherent limitations to this method: after deleting the pods, the
script must check at least once for the "Terminating" state before issuing a
request. This introduces a brief delay, resulting in a small underestimation of
the actual cold start time.
