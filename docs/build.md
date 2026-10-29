# How to build the DFaaS components

Prerequisite: you need to have [just](https://just.systems/) installed on your
system, and also [buildah](https://buildah.io/).

Each DFaaS component has a `Containerfile` in its root directory, which is also
used as build context. The available components are:

| Component                | Directory                              | Image name                      |
|--------------------------|----------------------------------------|---------------------------------|
| DFaaS Agent              | `dfaasagent`                           | `agent`                         |
| DFaaS Forecaster         | `forecaster`                           | `forecaster`                    |
| memstress function       | `functions/memstress`                  | `openfaas-memstress`            |
| image-classification     | `functions/image-classification`       | `openfaas-image-classification` |
| rejector                 | `k8s/scripts/rejector`                 | `rejector`                      |
| haproxy-metrics-exporter | `k8s/scripts/haproxy-metrics-exporter` | `haproxy-metrics-exporter`      |

To build the image of one component, you can run the following command:

```console
$ just build NAME DIR [ACTIONS...]
```

This builds `DIR/Containerfile` with `DIR` as build context and tags the image
as `NAME:dev` in the local registry (`localhost/NAME:dev`). After the build, it
runs the given actions in order:

* `k3s`: imports the image into the local k3s instance (see the `k3s` recipe),
* `push`: pushes the image to GitHub Container Registry (see the `push` recipe).

Without actions, the image is only built and tagged. Some examples:

```console
$ just build openfaas-memstress functions/memstress
$ just build rejector k8s/scripts/rejector k3s
$ just build openfaas-image-classification functions/image-classification push
```

Since the main components are the DFaaS Agent and Forecaster, there are two
following shortcuts:

```console
$ just agent k3s
$ just forecaster k3s push
```

The `k3s NAME` recipe imports the already built `NAME:dev` image into the local K3s instance. The tag stays
`NAME:dev`, and you can refer to that tag inside the cluster.

The `push NAME` recipe tags the already built `NAME:dev` image as
`ghcr.io/unimib-datai/dfaas-NAME:dev` and pushes it to GitHub Container
Registry. Make sure buildah is logged in to GHCR first:

```console
$ buildah login ghcr.io
$ just push agent
```
