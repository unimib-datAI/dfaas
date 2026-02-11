# Timeouts

This document describes how to configure timeout values for each component in
the DFaaS stack, including the client (k6).

The general rule for setting timeouts is:

    OpenFaaS Function < OpenFaaS Gateway < HAProxy < Client (k6)

This hierarchy helps prevent "zombie" requests in which an OpenFaaS Function
continues processing a request after it has already been cancelled by the client
or HAProxy.

Note: We use the OpenFaaS Community Edition, which has some limitations in the
Gateway. Specifically, we do not explicitly use a queue, and the gateway does
not provide per-function maximum concurrency. Timeout settings in the gateway
apply to all functions, not individually.

## OpenFaaS Function

OpenFaaS functions built with the classic
[watchdog](https://github.com/openfaas/classic-watchdog) or new
[of-watchdog](https://github.com/openfaas/of-watchdog) support the following
environment variables:

* `read_timeout`: Timeout for reading the payload from the client caller (in
  seconds, default is 30s).
* `write_timeout`: Timeout for writing the response body from your function (in
  seconds, default is 30s).
* `exec_timeout`: Hard timeout for the process executed for each request (in
  seconds, default is 0s; disabled if set to 0).
* `max_inflight`: Maximum number of requests in flight (default is 0; disabled).

Read and write timeouts primarily matter if the function's bottleneck is the
amount of data being received or sent. The exec timeout is the most important
setting, especially when CPU or RAM usage is growing or when handling high
concurrency. The max inflight setting relates closely to exec timeout by
controlling concurrent requests.

We recommend disabling readiness and liveness probes for OpenFaaS functions, and
using only `max_inflight` to control the number of concurrent requests per
function replica.

See additional environment variables in the watchdog's
[README](https://github.com/openfaas/classic-watchdog?tab=readme-ov-file#environment-variable-overrides)
or the of-watchdog's
[README](https://github.com/openfaas/of-watchdog?tab=readme-ov-file#configuration).

You can set these environment variables in two ways:

1. When deploying the function with `faas-cli`, by specifying the `--env
   ENVVAR=VALUE` option (repeat for each variable).
2. After deployment, by updating the Kubernetes deployment object associated
   with the function, using `kubectl edit` or `kubectl patch`.

> [!NOTE]
> Both classic watchdog and of-watchdog expose some metrics to Prometheus, but
> scraping is disabled by default. To enable metrics scraping, add the
> `prometheus.io.scrape` annotation set to `true` in the deployment object
> (using `faas-cli deploy --annotation` or `kubectl edit`).

## OpenFaaS Gateway

The OpenFaaS Gateway in the Community Edition is limited but does support these
environment variables:

* `read_timeout`: Timeout for reading a request from clients (in seconds,
  default is 60s).
* `write_timeout`: Timeout for writing a response from functions (in seconds,
  default is 60s).
* `upstream_timeout`: Maximum duration of HTTP call to upstream URL (in seconds,
  default is 60s). This must be smaller than `read_timeout` and `write_timeout`.

These timeouts apply to all deployed functions; it is not possible to specify
custom timeouts for individual functions. To set these environment variables,
use the same approaches as described for OpenFaaS Functions.

The gateway exports metrics to Prometheus, with scraping enabled by default.

## HAProxy

WIP

## k6

WIP
