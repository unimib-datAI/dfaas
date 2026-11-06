# noop OpenWhisk function

This folder contains the source code for the `noop` function in OpenWhisk. It
implements a basic Go Web Action that does no work and just replies with `200
OK`. It is meant to measure the overhead of the platform, without any noise from
the function itself.

This is the OpenWhisk port of the [OpenFaaS version](../../openfaas/noop). The
behavior is the same. The differences are:

* [main.go](main.go) contains the `Main(args)` entry point called by the
  OpenWhisk Go runtime and returns a Web Action response, instead of being an
  HTTP server.
* There is no Containerfile, since the function has no dependencies. The action
  runs on the official `go:1.23` runtime, which is already available in the
  [OpenWhisk chart](../../../k8s/charts/openwhisk/runtimes.json), so there is no
  image to build and push. For the same reason there is no `go.mod`: the runtime
  compiles the single source file when it initializes the action.

> [!WARNING]
> This action is compiled with Go 1.23 by the `openwhisk/action-golang-v1.23`
> runtime, while the [OpenFaaS version](../../openfaas/noop) uses Go 1.27. Go
> 1.23 is the latest version with an official OpenWhisk runtime image.

## Deploy

We assume the `wsk` CLI is configured with the administrator credentials (see
[the OpenWhisk chart README](../../../k8s/charts/openwhisk/README.md)), so the
action is created in the `whisk.system` namespace. Since the namespace is given
by the credentials, there is no need to specify it.

Create the action as a Web Action from the source file:

```console
$ wsk action create noop functions/openwhisk/noop/main.go \
    --kind go:1.23 --web true --memory 128 --annotation dfaas.maxrate 100
```

> [!IMPORTANT]
> Do not use hyphens or underscores, otherwise they will break the HAProxy
> generated configuration!

Note that DFaaS Agent considers only Web Actions. The `dfaas.maxrate` annotation
is required by the Recalc strategy. You can omit it if the DFaaS Agent does not
use this strategy. You can also set `dfaas.timeout_ms` to configure the
per-request timeout, or `timeout` for the Web Action timeout.

To update the code, run `wsk action update` with the same arguments.

## Usage

The function accepts requests with any method, path and body, and always replies
with `200 OK`. It has no parameters.

## Test

Use curl to send the requests (the action is in the `default` package of the
`whisk.system` namespace):

```console
$ curl -i http://127.0.0.1:30080/api/v1/web/whisk.system/default/noop
HTTP/1.1 200 OK
[...]

OK
```

Note that the endpoint is http://127.0.0.1:30080, the HAProxy public port, and
not directly the OpenWhisk API Gateway. The IP address should be adjusted to
match your local OpenWhisk instance.
