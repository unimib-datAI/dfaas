# memstress OpenFaaS function

This folder contains the source code for the `memstress` function in OpenFaaS.
It implements a basic Go function that allocates a given amount of memory,
holds it for a given number of seconds and then releases it. It is meant to
stress the memory of the nodes, especially with many concurrent requests.

The function logic can be found in [main.go](main.go). It is a plain Go HTTP
server listening on port `8081`.

## Build and push

Use the project's [justfile](../../justfile) to build the image:

```console
$ just build openfaas-memstress functions/memstress
```

This will generate the image with `localhost/openfaas-memstress:dev` tag. To
push the image to GitHub Container Registry (after login) with tag
`ghcr.io/unimib-datai/dfaas-openfaas-memstress`:

```console
$ just build openfaas-memstress functions/memstress push
```

> [!IMPORTANT]
> OpenFaaS CE does not support private images. A workaround is to publish the
> images to a public registry. See [this
> comment](https://github.com/unimib-datAI/dfaas/issues/32#issuecomment-3356311152)
> for more details.

Use the public image to deploy the function to OpenFaaS:

```console
$ faas-cli deploy --image=ghcr.io/unimib-datai/dfaas-openfaas-memstress:dev --name=memstress --label dfaas.maxrate=100
```

> [!IMPORTANT]
> Do not use hyphens or underscores, otherwise they will break the HAProxy
> generated configuration!

This will deploy the function to the local OpenFaaS instance as `memstress`.
The `dfaas.maxrate` label is required by the Recalc strategy. You can omit the
label if the DFaaS Agent does not use this strategy.

You can also pass the options `--env exec_timeout=60s` and `--env
max_inflight=400` to configure the per-request execution timeout and the
maximum number of concurrent requests each replica can handle. The execution
timeout must be longer than the largest `seconds` value you plan to send,
otherwise the request is cut off. See [docs/timeout.md](../docs/timeout.md) for
more details.

It is also recommended to set `GOMEMLIMIT` slightly below the container memory
limit (for example `--env GOMEMLIMIT=900MiB` for a 1 GiB limit). Near the
limit, the Go garbage collector runs more aggressively and returns memory to
the OS faster.

## Usage

The function accepts only `GET` requests with two optional query parameters:

| Parameter | Description                               | Default |
|-----------|-------------------------------------------|---------|
| `mb`      | Memory to allocate, in MiB (>= 1)         | `128`   |
| `seconds` | How long to hold the memory, in s (>= 0)  | `5`     |

A parameter that is present but invalid returns `400 Bad Request` with the
reason in the body.

## Test

Use curl to send the requests:

```console
$ curl "http://127.0.0.1:30080/function/memstress?mb=256&seconds=10"
requested_mib=256 duration_seconds=10 heap_alloc_mib=256
```

Note that the endpoint is http://127.0.0.1:30080, the HAProxy public port, and
not directly the OpenFaaS Gateway. The IP address should be adjusted to match
your local OpenFaaS instance.
