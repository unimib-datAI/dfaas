# memstress OpenWhisk function

This folder contains the source code for the `memstress` function in OpenWhisk.
It implements a basic Go Web Action that allocates a given amount of memory,
holds it for a given number of seconds and then releases it. It is meant to
stress the memory of the nodes, especially with many concurrent requests.

This is the OpenWhisk port of the [OpenFaaS version](../../openfaas/memstress).
The behavior is the same. The differences are:

* [main.go](main.go) contains the `Main(args)` entry point called by the
  OpenWhisk Go runtime and returns a Web Action response, instead of being an
  HTTP server.
* There is no Containerfile since the function has no dependencies. The action
  runs on the official `go:1.23` runtime, which is already available in the
  [OpenWhisk chart](../../../k8s/charts/openwhisk/runtimes.json), so there is no
  image to build and push. For the same reason there is no `go.mod`, the runtime
  compiles the single source file when it initializes the action.
* Each container handles one request at a time, since the [OpenWhisk
  chart](../../../k8s/charts/openwhisk/templates/_helpers.tpl) limits the action
  concurrency to 1. Concurrent requests are served by different containers.

> [!WARNING]
> This action is compiled with Go 1.23 by the `openwhisk/action-golang-v1.23`
> runtime, while the [OpenFaaS version](../../openfaas/memstress) uses Go 1.27.
> Go 1.23 is the latest version with an official OpenWhisk runtime image.

> [!WARNING]
> The function accepts up to 1024 MiB like the OpenFaaS version, but the
> [OpenWhisk chart](../../../k8s/charts/openwhisk/templates/_helpers.tpl) limits
> the action memory to 512 MiB. A request that exceeds the memory limit of the
> action fails because the container runs out of memory, so keep `mb` below the
> value given to `--memory`.

## Deploy

We assume the `wsk` CLI is configured with the administrator credentials (see
[the OpenWhisk chart README](../../../k8s/charts/openwhisk/README.md)), so the
action is created in the `whisk.system` namespace. Since the namespace is given
by the credentials, there is no need to specify it.

Create the action as a Web Action from the source file:

```console
$ wsk action create memstress functions/openwhisk/memstress/main.go \
    --kind go:1.23 --web true --memory 512 --timeout 60000 --annotation dfaas.maxrate 100
```

> [!IMPORTANT]
> Do not use hyphens or underscores, otherwise they will break the HAProxy
> generated configuration!

Note that DFaaS Agent considers only Web Actions. The `dfaas.maxrate` annotation
is required by the Recalc strategy. You can omit it if the DFaaS Agent does not
use this strategy. You can also set `dfaas.timeout_ms` to configure the
per-request timeout.

The `--timeout` option sets the action timeout in milliseconds. It must be longer
than the largest `seconds` value you plan to send, otherwise the request is cut
off.

To update the code, run `wsk action update` with the same arguments.

## Usage

The function accepts only `GET` requests with two optional query parameters:

| Parameter | Description                               | Default |
|-----------|-------------------------------------------|---------|
| `mb`      | Memory to allocate, in MiB (>= 1)         | `128`   |
| `seconds` | How long to hold the memory, in s (>= 0)  | `5`     |

A parameter that is present but invalid returns `400 Bad Request` with the
reason in the body.

## Test

Use curl to send the requests (the action is in the `default` package of the
`whisk.system` namespace):

```console
$ curl "http://127.0.0.1:30080/api/v1/web/whisk.system/default/memstress?mb=256&seconds=10"
requested_mib=256 duration_seconds=10 heap_alloc_mib=256
```

Note that the endpoint is http://127.0.0.1:30080, the HAProxy public port, and
not directly the OpenWhisk API Gateway. The IP address should be adjusted to
match your local OpenWhisk instance.
