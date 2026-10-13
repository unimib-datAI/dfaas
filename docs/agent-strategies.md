# Agent Strategies

The DFaaS agent can handle incoming requests using different strategies. These
strategies determine how to configure the HAProxy weights, allowing the agent to
forward requests to other agents, process them locally, or reject them.

Currently, there are the following strategies:

1. Recalc (`recalcstrategy`),
2. Node Margin (`nodemarginstrategy`),
3. Static (`staticstrategy`),
4. All Local (`alllocalstrategy`),
5. RL Agent (`rlagentstrategy`),
6. Random (`randomstrategy`),
7. Latency Threshold (`latencythresholdstrategy`).

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

The Recalc strategy is the first implemented load-balancing approach in DFaaS.
Each agent periodically recalculates the local processing, rejection, and
forwarding weights based mostly on real-time metrics and the current state of
the system.

The strategy uses the concept of maximum requests per second (`maxrate`), which
defines the maximum number of requests that a node can accept locally for a
specific function. This is a static and predefined parameter for each function
deployed on the node. The `maxrate` value is one of the inputs used by the
strategy to calculate the weights.

You must assign this label using the `faas-cli` command when deploying a
function to a node:

    faas-cli deploy --image=[...] --name=[...] --label dfaas.maxrate=100

Other inputs are taken from the monitoring platform:

- The real-time invocation rate for requests coming directly from clients.
- The real-time invocation rate for requests coming from each neighbor.

If the real-time invocation rate for requests coming directly from clients is
lower than the `maxrate`, the local node can also accept requests from other
neighbors, depending on the remaining capacity. The node sends a message to all
its neighbors to inform them about the available capacity.

On the other hand, if the real-time invocation rate for requests coming directly
from clients is higher than the `maxrate`, the function is considered
overloaded, and the proxy is configured to reject requests coming from other
neighbors. The local node also tries to forward requests coming from clients to
neighbors if capacity is available. Any excess requests that cannot be processed
or forwarded will be rejected.

The entire process is performed independently and periodicaly for each function.

The strategy specifically uses the `AGENT_RECALC_PERIOD` option to define how
often the strategy recalculates and shares the available capacity for forwarded
requests from other neighbors. An example could be `30s` for 30 seconds.

### Node Margin

> [!WARNING]
> Work in progress section! In the meantime, you can see the [strategy's source
> code](../agent/loadbalancer/nodemarginstrategy.go).

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

If you set the `dfaas.timeout_ms` label for a function (via `faas-cli` during
deployment), this strategy overrides the default HAProxy timeouts (connect and
server) with `dfaas.timeout_ms + 1s`. If the label is not provided, the default
timeouts are used (60 seconds).

This strategy updates the proxy configuration (and reloads the proxy) only when
there are changes to the deployed functions, such as when a new function is
added or an existing one is removed.

### RL Agent

The RL Agent strategy is an experimental, Reinforcement Learning-based approach
where request routing decisions are delegated to a trained model. Instead of
relying on predefined heuristics, the agent queries the model to determine the
weights for three possible actions: process locally, forward to neighbors, and
reject.

Unlike other strategies, RL Agent is event-driven rather than periodic. It
relies on the presence of the `DFaaS-K6-Stage` header in incoming requests,
which encodes the current stage of the workload.

The strategy works together with k6 load tests. Each k6 iteration consists of
two internal stages, so the iteration number is calculated by dividing the stage
value by 2. The RL model is queried once when a new iteration is detected.

The observation sent to the RL model combines information about the future
(predicted) iteration with measurements from the previous iteration. The future
iteration metrics are obtained from a historical Prometheus instance and are
used as a "holistic oracle" for the expected workload. The previous iteration
metrics are obtained from the real-time Prometheus instance. This means that
each DFaaS node has two Prometheus servers: one for real-time measurements and
one for historical measurements only. The latter must be deployed and managed
manually.

The observation for the model contains the following metrics:

* `input_rate`: input request rate for the current iteration.
* `previous_input_rate`: input request rate during the previous iteration.
* `previous_fwd_to_node_X`: request forwarding rate to each neighbor during the
  previous iteration.
* `previous_fwd_to_node_X_rejected`: rejected forwarding rate to each neighbor
  during the previous iteration.
* `reject_rate`: rejection rate for the current iteration.
* `previous_reject_rate`: rejection rate during the previous iteration.
* `avg_resp_time_loc`: average local response time for the current iteration.
* `previous_avg_resp_time_loc`: average local response time during the previous
  iteration.
* `previous_avg_resp_time_fwd_to_node_X`: average response time for requests
  forwarded to each neighbor during the previous iteration.
* `cpu_utilization`: CPU utilization for the current iteration.
* `previous_cpu_utilization`: CPU utilization during the previous iteration.
* `n_replicas`: number of replicas for the current iteration.
* `previous_n_replicas`: number of replicas during the previous iteration.

During the first iteration, previous-iteration metrics are not available. These
values are initialized to `0`, except for `previous_n_replicas`, which is
initialized to `1`.

The RL model returns a proportion for each available action: process requests
locally, forward requests to each neighbor, or reject requests. The returned
proportions are converted into HAProxy weights and applied.

To use this strategy, you must configure these additional variables:

* The RL model endpoint via the `AGENT_RLMODEL_HOST` and `AGENT_RLMODEL_PORT`
  environment variables.
* The `AGENT_RLMODEL_EXPLORE` option controls whether the RL model should
  explore the action space (by default, `false`).
* The historical Prometheus instance via the `AGENT_HISTORICAL_PROMETHEUS_HOST`
  and `AGENT_HISTORICAL_PROMETHEUS_PORT` environment variables. The query
  resolution is the same as the real-time Prometheus instance.

> [!IMPORTANT]
> The strategy has some limitations: it supports only one deployed function and,
> more importantly, dynamic addition or removal of neighbors or functions is not
> supported after the initial strategy configuration. Therefore, all functions
> and neighbors must be set up before starting the strategy. The strategy waits
> for 1 minute before starting, to allow all neighbors to connect.

### Random

The random strategy randomly selects how incoming requests are distributed
between the local node and the neighbors.

The strategy uses two options:

* `AGENT_RANDOM_SEED`: starting seed for the pseudo-random number generator. If
  set to `-1`, a random seed is used. The default value is `-1`.
* `AGENT_RANDOM_REJECT`: if set to `true`, the strategy can randomly reject
  requests instead of processing them locally or forwarding them to neighbors.
  The default value is `false`.

The strategy randomly generates a percentage of requests for each available
action:

* process requests locally;
* forward requests to neighbors, if available;
* reject requests, if `AGENT_RANDOM_REJECT` is set to `true`.

Each neighbor receives its own randomly generated percentage. The generated
percentages are then used as HAProxy weights. Therefore, different neighbors
can receive different amounts of incoming requests.

A new set of percentages is generated after every `AGENT_RECALC_PERIOD` for the
local node, each available neighbor, and, if enabled, the reject action.

If there are no neighbors and `AGENT_RANDOM_REJECT` is set to `false`, all
requests are processed locally. In this case, 100% of the requests are assigned
to the local node.

### Latency Threshold

The latency threshold strategy is a latency-aware strategy that selects
neighbors with a Round-Trip Time (RTT) lower than a user-configured threshold
and forwards requests to them instead of processing all requests locally. The
requests are divided equally between the local node and the selected neighbors.
If there are no suitable neighbors, all requests are processed locally.

The strategy uses the `AGENT_LATENCY_THRESHOLD_MS` option as the latency
threshold when evaluating neighbors. The latency is measured using an RTT ping
to each neighbor. Three pings are performed and their results are averaged to
get a more stable latency value. This process is repeated every
`AGENT_RECALC_PERIOD`.

When `AGENT_LATENCY_THRESHOLD_MS` is set to 0ms, the strategy behaves exactly as
the All Local strategy.
