# k6

To automatically perform load tests on one or more DFaaS nodes, we use
[**k6**](https://grafana.com/oss/k6/) and a custom Python script (using
*matplotlib*) to produce plots. Each test is defined and configured through a
custom JavaScript file.

It may be helpful to refer to the [official k6
documentation](https://grafana.com/docs/k6/latest/).

## How to run

To execute a test, run k6 as follows:

```console
$ k6 run single_node_test.js --out json=result.json
```

You may need to manually edit the JavaScript file to specify the IP addresses of
the DFaaS nodes and the function endpoint(s).

We assume k6 is installed locally. For other installation or execution options
(via Podman or Docker), see the [official k6
documentation](https://grafana.com/docs/k6/latest/set-up/install-k6/).

After execution, k6 produces a summary on standard output and a JSON file
containing real-time metrics (see the [k6
documentation](https://grafana.com/docs/k6/latest/results-output/real-time/json/)
for details). You can analyze this JSON file using tools like
[jq](https://jqlang.org/) or [fx](https://fx.wtf/), or generate plots using the
provided Python script.

To run the Python script:

```console
$ WIP
```

As with the test definition, the Python script is tailored for a specific test.
You may need to modify it for your use case. Before running it, set up a virtual
environment as follows:

```console
$ sudo apt install python3-venv # On Ubuntu
$ python3 -m venv .env
$ source .env/bin/activate
$ pip install --requirement requirements.txt
```

## Predefined tests

* "Single node" ([`single_node.js`](single_node.js)): By default, it sends 110
  requests per second to the figlet function on the local DFaaS node for 3
  minutes. This test was originally designed to evaluate the Recalc strategy
  using the figlet function, with a maximum rate of 100 requests per second
  (defined when the function has been deployed). As a result, the node must
  either forward requests to other nodes or reject them if no additional nodes
  are available.
* "Three nodes" (WIP): Three parallel load tests are executed on three DFaaS
  nodes, each with a different request-per-second rate, start delay, and
  duration. The figlet function is the target of these tests. These tests were
  originally used with the old Operator component.

## Old operator

An older, unsupported version of the Operator component used
[Vegeta](https://github.com/tsenart/vegeta) for load testing DFaaS nodes. It has
since been replaced by k6, which supersedes both Vegeta and the custom Bash
script used to run it. The Python plotting script was updated accordingly, and
Docker images for the old operator are no longer built or published. While the
legacy images [remain
available](https://github.com/unimib-datAI/dfaas/pkgs/container/dfaas-operator),
they are no longer supported.
