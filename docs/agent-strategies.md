# Agent Strategies

The DFaaS agent can handle incoming requests using different strategies. These
strategies determine how to configure the HAProxy weights, allowing the agent to
forward requests to other agents, process them locally, or reject them.

Currently, there are the following strategies:

1. Recalc (`recalcstrategy`),
2. Node Margin (`nodemarginstrategy`),
3. Static (`staticstrategy`),
4. All Local (`alllocalstrategy`).

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
`AGENT_RECALC_PERIOD` option. The recalculation process involves two phases:

1. The agent gathers statistics and updates the local state.  
2. It then calculates new weights and applies them to the HAProxy configuration.

Between the two phases, there is a pause of `AGENT_RECALC_PERIOD`/2.

The following metrics are collected and used:

* List of connected nodes with their supported functions and status (to
  determine how many requests the local node can forward to neighbors).  
* List of functions with the associated maximum rate for the local node.  
* Function invocation rates for the local node over the previous 1 second.

Among these metrics, the most important is the last one, as it determines the
overload status for each function. Function invocation rates are compared with
the corresponding maximum rate, and if the rate is higher the function is
considered overloaded. If underloaded, the "margin" (the capacity to accept
requests from users or other nodes) is calculated for each function and evenly
distributed among all neighbors. In phase 2, the HAProxy weights are then
computed and applied based on this information.

At the end of each iteration, each node sends the status of each function to all
other nodes.

The Recalc strategy uses the concept of **maxrate** (maximum requests per
second) for each function to determine how many requests a node can handle for
that function. This maxrate is a fixed label and must be assigned to every
deployed function. You can assign this label using the faas-cli command when
deploying a function to a node:

```console
faas-cli deploy --image=[...] --name=[...] --label dfaas.maxrate=100
```

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

### All Local

This is a simple, baseline strategy that always forwards requests to the local
node, the local OpenFaaS Gateway instance. It supports only the
`AGENT_RECALC_PERIOD` option.

Tthis strategy updates the proxy configuration (and reloads the proxy) only when
there are changes to the deployed functions, such as when a new function is
added or an existing one is removed.
