# Commands

This is a short list of useful commands for a development environment. Warning:
this section is still a work in progress.

Deploy Helm charts using a fixed version:

```console
$ sudo helm install haproxy haproxytech/haproxy --values k8s/charts/values-haproxy.yaml --version 1.26.1
$ sudo helm install prometheus prometheus-community/prometheus --values k8s/charts/values-prometheus.yaml --version 27.37.0
$ sudo helm install openfaas openfaas/openfaas --values k8s/charts/values-openfaas.yaml --version 14.2.128
```

Check an HAProxy configuration:

```console
$ podman run --rm -v ./LOCAL_CONFIG.cfg:/etc/haproxy.cfg docker.io/library/haproxy:latest haproxy -c -f /etc/haproxy.cfg
```

Push a new HAProxy configuration (stored in `LOCAL_CONFIG.cfg`) through the Data
Plane API:

```console
$ curl -X POST -u admin:admin -H "Content-Type: text/plain" --data-binary @LOCAL_CONFIG.cfg 'http://localhost:30555/v3/services/haproxy/configuration/raw?skip_version=true&force_reload=true'
```
