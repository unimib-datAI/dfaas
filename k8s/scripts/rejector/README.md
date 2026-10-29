# Rejector

This small web server rejects all incoming requests. See the comments in the
source code [`main.go`](main.go) for more information.

The program supports only the `REJECTOR_HTTP_STATUS_CODE` environment variable
as configuration.

## Build

You need the Go toolchain. To build and run the binary, just run:

```console
$ go build .
$ ./rejector
```

If you want to deploy to a Kubernetes cluster, you can use the given Helm chart:

    $ sudo helm install rejector ./helm/

See the [`values.yaml`](helm/values.yaml) file for more information.

You can build (and push the local k3s cluster) the container image with the
project's [justfile](../../../justfile):

    $ just build rejector k8s/scripts/rejector k3s
