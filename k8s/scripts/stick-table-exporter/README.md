# stick-table-exporter

stick-table-exporter is a small Go service that continuously polls a stick-table
value from HAProxy via the HAProxy Data Plane API and exposes the collected
samples over HTTP.

The main goal is to allow the synchronization of load testing stages between k6
(client side) and a DFaaS node (server side).

See the comments in the source code [`main.go`](main.go) for more information.

## Build

You need the Go toolchain. To build and run the binary, just run:

```console
$ go build .
$ ./stick-table-exporter
```

If you want to deploy to a Kubernetes cluster, you can use the given Helm chart:

    $ sudo helm install ./helm/

See the [`values.yaml`](helm/values.yaml) file for more information. In both
cases, the program assumes that the HAProxy Data Plane API is reachable and the
admin credentials are `admin:admin`.

You can build (and push the local k3s cluster) the container image with the
[`k8s/scripts/build-image.sh`](k8s/scripts/build-image.sh) script:

    $ k8s/scripts/build-image.sh stick-table-exporter k3s --dockerfile k8s/scripts/stick-table-exporter/Containerfile

Make sure to be in the project's root directory, not in
k8s/scripts/stick-table-exporter directory!

## Exposed HTTP endpoint

The program exposes the GET `/table` endpoint that returns the stage index for
each second (as UNIX timestamp) with `application/json` or `text/csv` format.

Example request with curl:

```console
$ curl 'localhost:30808/table' -H "Accept: text/csv"
timestamp,stage
1771423897,-1
1771423898,-1
1771423899,-1
...
```

The `-1` value means that for this timestamp there is no stage index available.

The endpoint supports optional `start` and `end` query parameters, they must be UNIX
timestamps in seconds:

* No `start` and no `end`: return all buffered samples,
* Only `start`: return samples from start to now,
* Only `end`: return samples from oldest buffered sample up to end,
* Both `start` and `end`: return samples within the inclusive range [start,
  end].

## Configuration

Configuration is done via environment variables. See the source code or the Helm
chart for more information.
