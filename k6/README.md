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
$ k6 run single_node_test.js --out csv=results.csv.gz
```

You must check the test script to see which custom environment variables it
supports. You may also need to modify the script to adjust it to your needs (for
example, setting

We assume k6 is installed locally. For other installation or execution options
(via Podman or Docker), see the [official k6
documentation](https://grafana.com/docs/k6/latest/set-up/install-k6/).

After execution, k6 produces a summary on standard output and a compressed CSV
file containing real-time metrics (see the [k6
documentation](https://grafana.com/docs/k6/latest/results-output/real-time/json/)
for details). You can analyze this CSV file using Python with
[pandas](https://pandas.pydata.org/).

Note: we recommend to output the Gzipped version of the CSV file. To get the
original CSV file, use `zcat`. Note that pandas supports compressed CSV files.

## Predefined tests

* "Single node" ([`single_node.js`](single_node.js)): By default, it sends 110
  requests per second to the figlet function on the local DFaaS node for 3
  minutes. This test was originally designed to evaluate the Recalc strategy
  using the figlet function, with a maximum rate of 100 requests per second
  (defined when the function has been deployed). As a result, the node must
  either forward requests to other nodes or reject them if no additional nodes
  are available.
* "Three nodes" ([`three_nodes.js`](three_nodes.js)): Three parallel load tests
  are executed on three DFaaS nodes, each with a different request-per-second
  rate, start delay, and duration. The figlet function is the target of these
  tests. These tests were originally used with the old Operator component.

## Advanced k6 run

You can enable the live web dashboard and generate an HTML report by setting the
respective environment variables when running k6:

    $ K6_WEB_DASHBOARD=true K6_WEB_DASHBOARD_PORT=30665 K6_WEB_DASHBOARD_EXPORT=k6_report.html k6 run single_trace.js --out csv=k6_results.csv.gz

The `single_trace.js` test script supports the following additional environment
variables:

* `TRACE_PATH`: Path to the traces in JSON format.
* `FUNCTION`: Function ID to filter from the traces.
* `NODE`: Node ID to filter from the traces.
* `LIMIT`: Number of initial steps to consider. Use `0` to disable the limit
  (process the full trace).

The combination of `FUNCTION` and `NODE` allows you to select a specific trace.
Only the `TRACE_PATH` variable is mandatory.

You can also pass environment variables directly via k6:

    $ k6 run single_trace.js --env TRACE_PATH=input_requests_scaled_traces.json

Since there are multiple environment variables, we recommend using **direnv** to
automatically load them from a file when running k6. All variables listed above
are already defined and populated in the `.envrc` file in this directory. For
installation and configuration instructions, refer to the [official direnv
documentation](https://direnv.net/).

## Operating system and hardware limits

When running k6 with a high number of requests per second, the tool can consume
a large number of virtual users (VUs). In this situation, k6 may report the
error "Too Many Open Files". To resolve this, you may need to fine-tune Linux
system settings according to the [official k6
documentation](https://grafana.com/docs/k6/latest/set-up/fine-tune-os/) and
increase the available CPU and RAM resources.

You can apply the recommended settings by using the custom Ansible playbook
located in this directory ([`fine-tune-client.yaml`](fine-tune-client.yaml)). We
assume that Ansible is executed on the same host where k6 is running:

    $ ansible-playbook --inventory localhost, --connection local fine-tune-client.yaml

After running the playbook, reboot the host to ensure that all changes take
effect. If k6 is running on a different node, you will need to create an
`inventory.yaml` file that includes the target host information, the user, and
the root password, and then provide this file to Ansible.

An Ansible playbook is also available for fine-tuning a DFaaS node, named
[`fine-tune-server.yaml`](fine-tune-server.yaml). You will need to reboot the
node after the playbook.

## Old operator

An older, unsupported version of the Operator component used
[Vegeta](https://github.com/tsenart/vegeta) for load testing DFaaS nodes. It has
since been replaced by k6, which supersedes both Vegeta and the custom Bash
script used to run it. The Python plotting script was updated accordingly, and
Docker images for the old operator are no longer built or published. While the
legacy images [remain
available](https://github.com/unimib-datAI/dfaas/pkgs/container/dfaas-operator),
they are no longer supported.
