# Agent Strategies

The DFaaS agent can handle incoming requests using different strategies. These
strategies determine how to configure the HAProxy weights, allowing the agent to
forward requests to other agents, process them locally, or reject them.

Currently, there are three strategies:

1. Recalc (`recalcstrategy`),
2. Node Margin (`nodemarginstrategy`),
3. Static (`staticstrategy`).

## How to set the strategy

When started, the DFaaS agent reads the `AGENT_STRATEGY` environment variable.
If not set, it defaults to Recalc. You can specify a strategy by setting this
variable directly. Alternatively, if you are using the provided Helm chart, you
can set the strategy by creating a custom values.yaml file with the following
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

> [!WARNING]
> Work in progress section!

### Recalc

### Node Margin

### Static
