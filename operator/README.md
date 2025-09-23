# Operator

The **Operator** container automates the execution of load tests using
[Vegeta](https://github.com/tsenart/vegeta) on DFaaS nodes. Its workflow is
managed by the [operator.sh](./docker/operator.sh) script.

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

**Note:** before running the Operator, ensure that
[plot-results.py](docker/plot-results.py) is configured to properly generate
plots for both individual and merged results.

## How to run

> [!IMPORTANT]
> You must deploy at least one DFaaS node before starting the operator!

You need to configure the operator using a `.env` file, like the default
[operator.env](operator.env). Options includes:

- **Target nodes**: Endpoints (IP and port pairs) to run tests against.
- **Vegeta attack configuration**: For each attack, specify:
  - Delay before starting the attack
  - Duration
  - Rate (requests per second)
  - HTTP request specification (method, URL, headers, body, etc.)

Optionally modify [plot-results.py](docker/plot-results.py) if custom plotting
behavior is needed, but you'll need to rebuild the image.

Then, use podman or Docker to run the container in background:

```console
$ podman run --detach --env-file operator.env --volume $(pwd)/test:/test ghcr.io/unimib-datai/dfaas-operator:dev
$ podman logs --follow <container_id_or_name>
```

Or run the container as a foreground process:

```console
$ podman run --interactive --tty --env-file operator.env --volume $(pwd)/tests:/tests ghcr.io/unimib-datai/dfaas-operator:dev
$ # Or use the short options format -it instead of --interactive and --tty.
```

## Build

Use the help script to build the image from the project's root directory:

```console
$ ./k8s/scripts/build-image.sh operator none --dockerfile operator/Dockerfile
```

This will generate the image with `localhost/operator:dev` tag. To push the
image to GitHub Container Registry (after login) with tag
`ghcr.io/unimib-datai/dfaas-operator`:

```console
$ ./k8s/scripts/build-image.sh operator push --dockerfile operator/Dockerfile
```
