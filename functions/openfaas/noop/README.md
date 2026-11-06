# noop OpenFaaS function

This folder contains the source code for the `noop` function in OpenFaaS. It
implements a basic Go function that does no work and just replies with `200 OK`.
It is meant to measure the overhead of the platform, without any noise from the
function itself.

The function logic can be found in [main.go](main.go). It is a plain Go HTTP
server listening on port `8090`.

## Build and push

Use the project's [justfile](../../../justfile) to build the image:

```console
$ just build openfaas-noop functions/openfaas/noop
```

This will generate the image with `localhost/openfaas-noop:dev` tag. To push the
image to GitHub Container Registry (after login) with tag
`ghcr.io/unimib-datai/dfaas-openfaas-noop`:

```console
$ just build openfaas-noop functions/openfaas/noop push
```

> [!IMPORTANT]
> OpenFaaS CE does not support private images. A workaround is to publish the
> images to a public registry. See [this
> comment](https://github.com/unimib-datAI/dfaas/issues/32#issuecomment-3356311152)
> for more details.

Use the public image to deploy the function to OpenFaaS:

```console
$ faas-cli deploy --image=ghcr.io/unimib-datai/dfaas-openfaas-noop:dev --name=noop --label dfaas.maxrate=100
```

> [!IMPORTANT]
> Do not use hyphens or underscores, otherwise they will break the HAProxy
> generated configuration!

This will deploy the function to the local OpenFaaS instance as `noop`. The
`dfaas.maxrate` label is required by the Recalc strategy. You can omit the label
if the DFaaS Agent does not use this strategy.

You can also pass the options `--env exec_timeout=60s` and `--env
max_inflight=400` to configure the per-request execution timeout and the maximum
number of concurrent requests each replica can handle. See
[docs/timeouts.md](../../../docs/timeouts.md) for more details.

## Usage

The function accepts requests with any method, path and body, and always replies
with `200 OK`. It has no parameters.

## Test

Use curl to send the requests:

```console
$ curl -i http://127.0.0.1:30080/function/noop
HTTP/1.1 200 OK
[...]

OK
```

Note that the endpoint is http://127.0.0.1:30080, the HAProxy public port, and
not directly the OpenFaaS Gateway. The IP address should be adjusted to match
your local OpenFaaS instance.
