# DFaaS overview

## High-level components of a single DFaaS node

* DFaaS Agent
* Reverse proxy (HAProxy)
* FaaS platform (OpenFaaS)
* Metrics (Prometheus)

## Low-level components of a single DFaaS node

Single DFaaS node: a single Kubernetes (k3s) cluster composed of a single node.

| Component        | Deployment/Chart  | Pods | Containers |
|------------------|-------------------|------|------------|
| DFaaS Agent      | DFaaS Agent       | 1    | 1          |
| DFaaS Agent      | DFaaS Forecaster  | 1    | 1          |
| Proxy            | HAProxy           | 1    | 1          |
| FaaS Platform    | OpenFaaS          | 1    | 2          |
| Metrics          | Prometheus        | 4    | 5          |

Note: in addition there are the individual FaaS functions (with zero or more replicas).

### Proxy

* Reverse proxy used: HAProxy
* Proxy used directly in the Kubernetes cluster: no Ingress/Gateway objects.
* Main needs: connect DFaaS agents and perform request routing with weights (for forwarding/processing/rejection).

Improvement proposals:

* Stay with HAProxy: there is no reason to change, it works well.
* Avoid restarting HAProxy too frequently to reload the configuration: use the REST API to update weights/backends. Only restart if strictly necessary (e.g., adding a neighbor).

### FaaS Platform

* Platform used: OpenFaaS
* Main flow: User -> Proxy -> Gateway (OpenFaaS) -> Function (and vice versa for the response)
* Requests are synchronous: there is no intermediate queue. As a consequence, in case of cold start and overload, request losses can occur.
* Limitations of OpenFaaS Community Edition: max 5 replicas per function, no CRD for functions (must use the Gateway REST API), limited metrics

Improvement proposals:

* Replace OpenFaaS with OpenWhisk
* Migration advantages: request queueing (Kafka), fewer limitations tied to premium/pro features, additional metrics
* Disadvantages: still no CRD for functions (in OpenFaaS it's available in pro), introduction of a request queue (Kafka), migration effort required

### Metrics

* Platform used: Prometheus
* Collected metrics (every 5s): DFaaS agent, OpenFaaS (used by OpenFaaS for scaling decisions), cAdvisor (integrated in k3s), node exporter
* Metrics used by the agent^: node exporter, OpenFaaS and cAdvisor

^ depends on the offloading strategy

## Data flow

Without forwarding:

```mermaid
sequenceDiagram
    actor U as User

    box transparent DFaaS
        participant P as Reverse Proxy
        participant GF as Gateway FaaS
        participant FF as FaaS Function
    end

    Note right of FF: Function already instantiated!

    U ->> +P: figlet("Hello")

    P ->> P: Check forwarding table

    P ->> +GF: figlet("Hello")

    GF ->> +FF: figlet("Hello")

    FF ->> FF: Execute function

    FF ->> -GF: response "HELLO"

    GF ->> -P: response "HELLO"

    P ->> -U: response "HELLO"
```

With forwarding:

```mermaid
sequenceDiagram
    actor U as User

    box transparent DFaaS (node A)
        participant PA as Reverse Proxy A
    end

    box transparent DFaaS (node B)
        participant P as Reverse Proxy B
        participant GF as Gateway FaaS
        participant FF as FaaS Function
    end

    Note right of FF: Function already instantiated!

    U ->> +PA: figlet("Hello")

    PA ->> PA: Check forwarding table

    PA ->> +P: figlet("Hello")

    P ->> +GF: figlet("Hello")

    GF ->> +FF: figlet("Hello")

    FF ->> FF: Execute function

    FF ->> -GF: response "HELLO"

    GF ->> -P: response "HELLO"

    P ->> -PA: response "HELLO"

    PA ->> -U: response "HELLO"
```

## Control flow

Periodic weight recalculation:

```mermaid
sequenceDiagram
    box transparent DFaaS (node A)
        participant A as DFaaS Agent
        participant F as Forecaster
        participant P as Reverse Proxy
        participant GF as Gateway FaaS
        participant M as Metrics
    end

    box transparent DFaaS (node B)
        participant AB as DFaaS Agent B
    end


    A ->> +A: Recalculation timer

    opt Strategy requires forecasting?
        A ->> F: GetForecast
        F ->> A: Forecast
    end

    A ->> GF: Get functions
    GF ->> A: List of functions
    
    A ->> M: Get metrics
    M ->> A: Metrics

    A ->> A: Compute weights

    A ->> P: Update configuration
    P ->> A: Configuration updated

    A ->> AB: Send forwarding weights/metrics

    deactivate A
```

## Offloading strategies implemented in DFaaS

The strategies determine the computation of request handling weights (forwarding, local processing, or rejection).

### Recalc Strategy

* Introduced in the thesis "Study and implementation of a peer-to-peer architecture for decentralized execution of FaaS functions" by Davide Motterlini
* Partially working: generally works but there are two small bugs to fix (see unimib-datAI/dfaas/issues/48 and unimib-datAI/dfaas/issues/45)
* Based on a static limit (`maxrate`) per function to determine over/underload state.

### Node Margin Strategy

* Introduced in the paper "Analysis and Evaluation of Load Management Strategies in a Decentralized FaaS Environment: A Simulation-Based Framework" by Filippini... (derived from two theses)
* Partially working: the issue is in the forecasting models which are not accurate
* Requires the forecasting component (`dfaas-forecaster`).

### Static Strategy

* Baseline with static weights: 60% probability of local processing, 40% probability of forwarding (if there is at least one neighbor) equally distributed.
* Working

## Running experiments

* Traffic generator: k6
* The experiment (node setup, traffic generation and reporting) is manual

## How DFaaS nodes are connected

* Physically: mesh network (?)
* Logically: p2p network, all DFaaS nodes are connected to each other
* Forwarding occurs to all nodes in the p2p network; there is no concept of proximity/latency
* DFaaS agents exchange periodic direct messages (control flow)

Improvement proposals:

* Add latency awareness
* Split the logical network into data and control:
    * Control network: remains p2p among all nodes
    * Data network: a node forwards only to nearby nodes (determined statically or dynamically via latency)
