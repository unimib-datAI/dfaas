# Operator

The **Operator** container automates the deployment of functions to DFaaS nodes
and executes load tests using [Vegeta](https://github.com/tsenart/vegeta). Its
workflow is managed by the
[operator_entrypoint.sh](./docker/files/operator_entrypoint.sh) script.

## Overview

The Operator performs the following tasks:

1. **Health Checks**: Verifies the status of target DFaaS nodes.
2. **Function Deployment**: Deploys specified functions to the healthy nodes.
3. **Proxy Reconfiguration Wait**: Waits for a configurable period to allow node
   proxies to register new function endpoints.
4. **Load Testing**: Launches one or more parallel Vegeta attacks according to
   the configuration.
5. **Results Visualization**: Generates plots for individual and merged attack
   results using [plot-results.py](docker/files/plot-results.py).
6. **Cleanup (Optional)**: Deletes deployed functions from nodes after the
   tests, if configured.

## Configuration

All Operator parameters are set in the [dfaasoperator.env](dfaasoperator.env)
file, including:
- **Target nodes**: IP addresses to run tests against.
- **Functions to deploy**: List of functions handled by the operator.
- **Vegeta attack configuration**: For each attack, specify:
  - Delay before starting the attack
  - Duration
  - Rate (requests per second)
  - HTTP request specification (method, URL, headers, body, etc.)

## Load Testing & Results

- The Operator supports running multiple Vegeta attacks in parallel, each with
  independently configured start delays, durations, and rates.
- For each attack, it generates performance plots using
  [plot-results.py](docker/files/plot-results.py).
- After all attacks complete, their `results.json` files are merged into a
  single `merged-results.json`. Aggregated plots are then produced from this
  file.

**Note:**  Before running the Operator, ensure that
[plot-results.py](docker/files/plot-results.py) is configured to properly
generate plots for both individual and merged results.

## Cleanup

If enabled in the environment file, the Operator will delete the deployed
functions from the DFaaS nodes after finishing load tests.

## Quick Start

1. Edit [dfaasoperator.env](dfaasoperator.env) to set up your target nodes,
   functions, and Vegeta attacks.
2. Configure [plot-results.py](docker/files/plot-results.py) if custom plotting
   behavior is needed.
3. Build and run the Operator container.

## References

- [Vegeta - HTTP load testing tool](https://github.com/tsenart/vegeta)
