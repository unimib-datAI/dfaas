# Operator

The **Operator** container automates the deployment of functions to DFaaS nodes
and executes load tests using [Vegeta](https://github.com/tsenart/vegeta). Its
workflow is managed by the [operator.sh](./docker/operator.sh) script.

The Operator performs the following tasks:

1. **Health Checks**: Verifies the status of target DFaaS nodes.
2. **Load Testing**: Launches one or more parallel Vegeta attacks according to
   the configuration.
3. **Results Visualization**: Generates plots for individual and merged attack
   results using [plot-results.py](docker/plot-results.py).

The Operator supports running multiple Vegeta attacks in parallel, each with
independently configured start delays, durations, and rates.

For each attack, it generates performance plots using
[plot-results.py](docker/plot-results.py).

After all attacks complete, their `results.json` files are merged into a single
`merged-results.json`. Aggregated plots are then produced from this file.

**Note:**  Before running the Operator, ensure that
[plot-results.py](docker/plot-results.py) is configured to properly generate
plots for both individual and merged results.

## How to run

You need to configure the operator using a `.env` file, like the default
[dfaasoperator.env](dfaasoperator.env). Options includes:

- **Target nodes**: IP addresses to run tests against.
- **Vegeta attack configuration**: For each attack, specify:
  - Delay before starting the attack
  - Duration
  - Rate (requests per second)
  - HTTP request specification (method, URL, headers, body, etc.)

Optionally modify [plot-results.py](docker/plot-results.py) if custom plotting
behavior is needed, but you'll need to rebuild the image.

Then, use podman or Docker to run the container:

```console
$ podman run -d --env-file operator.env -v $(pwd)/test:/test operator:dev
$ podman logs <container_id_or_name> -f
```

## Build

Use the help script to build the image from the project's root directory:

```console
$ ./k8s/scripts/build-image operator none --dockerfile operator/Dockerfile
```

Push the image to GitHub Container Registry (after login):

```console
$ ./k8s/scripts/build-image operator push --dockerfile operator/Dockerfile
```
