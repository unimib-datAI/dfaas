# haproxy-metrics-exporter

haproxy-metrics-exporter is a small Go service that exports additional metrics
extracted from HAProxy logs to Prometheus. In particular, it exposes the
`haproxy_custom_total_request_duration_seconds` histogram, providing
fine-grained statistics about the total latency of each request (including the
matching response, if available). This corresponds to the `Ta` timing event in
HAProxy.

This service is intended to run as a sidecar within the HAProxy pod. HAProxy
should be configured to forward logs to this service using the HTTP log format
(`option httplog`). The reason this service exists is that HAProxy by default
only exposes `haproxy_server_response_time_average_seconds`, which is a moving
average limited to the most recent 1024 successful connections.

Along with timing data, each request is also labeled with the corresponding
`backend`, `server`, and HTTP `status` code of the response.

## Build

You need a Go toolchain, at least version 1.26. To build and run the binary,
just run:

```console
$ go build .
$ ./haproxy-metrics-exporter
```

You can build (and push the local k3s cluster) the container image with the
[`k8s/scripts/build-image.sh`](k8s/scripts/build-image.sh) script:

    $ k8s/scripts/build-image.sh haproxy-metrics-exporter k3s --containerfile k8s/scripts/haproxy-metrics-exporter/Containerfile

Make sure to be in the project's root directory, not in
`k8s/scripts/haproxy-metrics-exporter` directory!

## Configuration

Configuration is done via CLI flags:

* `-logs-port`: UDP port used to listen for incoming HAProxy logs (default:
  `5014`),
* `-metrics-port`: TCP port used to expose the Prometheus HTTP server with the
  `/metrics` endpoint (default: `2112`).
