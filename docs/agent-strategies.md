# Agent Strategies

The DFaaS agent can handle incoming requests using different strategies. These
strategies determine how to configure the HAProxy weights, allowing the agent to
forward requests to other agents, process them locally, or reject them.

Currently, there are three strategies:

1. Recalc (`recalcstrategy`),
2. Node Margin (`nodemarginstrategy`),
3. Static (`staticstrategy`).

> [!TIP]
> For implementation details, refer to the code comments in the
> [loadbalancer](../dfaasagent/agent/loadbalancer) package, starting with
> [`strategyfactor.go`](../dfaasagent/agent/loadbalancer/strategyfactor.go).

## How to set the strategy

When started, the DFaaS agent reads the `AGENT_STRATEGY` environment variable.
If not set, it defaults to Recalc. You can specify a strategy by setting this
variable directly. Alternatively, if you are using the provided Helm chart, you
can set the strategy by creating a custom `values.yaml` file with the following
content:

```yaml
config:
  AGENT_STRATEGY: "staticstrategy"
```

Next, run Helm to install the DFaaS agent:

```console
$ sudo helm install dfaas-agent ./k8s/charts/agent --values values.yaml
```

You can redeploy the agent by reinstalling the chart, but you must first remove
the existing one:

```console
$ sudo helm uninstall dfaas-agent
```

A strategy may support certain options that should be provided as environment
variables.

## Avalable strategies

### Recalc

The Recalc strategy is a basic dynamic load balancing approach in which each
agent periodically recalculates the forwarding weights according to real-time
metrics and the state of the system.

The weights recalculation occurs at intervals defined by the
`AGENT_RECALC_PERIOD` option. The recalculation process involves:

1. The agent gather statistics and update local state,
2. Then it calculates new weights and apply them to the HAProxy configuration.

The following metrics are collected and used:

* Function invocation rates (requests per second for each function),
* CPU and RAM usage,
* Average function execution times,
* Node overload status,
* Service count (active requests to functions per node),
* Per-function CPU usage,
* Per-function RAM usage.

The Recalc strategy uses the concept of **maxrate** (maximum requests per
second) for each function to determine how many requests a node can handle for
that function. This maxrate is a fixed label and must be assigned to every
deployed function. You can assign this label using the faas-cli command when
deploying a function to a node:

```console
faas-cli deploy --image=[...] --name=[...] --label dfaas.maxrate=100
```

The maxrate value is used to detect overload (when a node's invocation rate for
a function approaches or exceeds its maxrate) and capacity (when the invocation
rate is well below the maxrate, indicating the node can accept more requests).

### Node Margin

> [!WARNING]
> Work in progress section!

### Static

In the static strategy, fixed weights are used to forward requests between
agents. It only supports the `AGENT_RECALC_PERIOD` option.

The exact logic for weights is as follows:

* 60% of incoming requests are processed locally by the node.
* 40% of requests are forwarded to neighbors and divided evenly among them.
* If there are no neighbors, all requests are processed locally.
