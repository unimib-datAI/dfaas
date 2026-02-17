# Commands

This is a short list of useful commands for a development environment. Warning:
this section is still a work in progress.

## Deploy the entire DFaaS node

```console
$ sudo helm install haproxy haproxytech/haproxy --values k8s/charts/values-haproxy.yaml --version 1.26.1
$ sudo helm install prometheus prometheus-community/prometheus --values k8s/charts/values-prometheus.yaml --version 27.37.0
$ sudo helm install openfaas openfaas/openfaas --values k8s/charts/values-openfaas.yaml --version 14.2.128
$ sudo helm install agent ./k8s/charts/agent/ --values values-test.yaml
```

Note that each chart has a fixed version and the `agent` chart requires a
`values-test.yaml` file for custom configuration.

## Lint an HAProxy config

```console
$ podman run --rm -v ./LOCAL_CONFIG.cfg:/etc/haproxy.cfg docker.io/library/haproxy:latest haproxy -c -f /etc/haproxy.cfg
```

## Interact with HAProxy Data Plane API

We suppose that the HAProxy Data Plane API endpoint is `localhost:30555` and the
credentials are `admin:admin` (username:password).

### Configuration

Push a new HAProxy configuration stored in `LOCAL_CONFIG.cfg`:

```console
$ curl -X POST -u admin:admin -H "Content-Type: text/plain" --data-binary @LOCAL_CONFIG.cfg 'http://localhost:30555/v3/services/haproxy/configuration/raw?skip_version=true&force_reload=true'
```

Save the current HAProxy configuration to `LOCAL_CONFIG.cfg`:

```console
$ curl -u admin:admin http://127.0.0.1:30555/v3/services/haproxy/configuration/raw --output LOCAL_CONFIG.cfg
```

Only validate the configuration (alternative to `podman run`) stored as
`LOCAL_CONFIG.cfg`:

```console
$ curl -X POST -u admin:admin -H "Content-Type: text/plain" --data-binary @LOCAL_CONFIG.cfg 'http://localhost:30555/v3/services/haproxy/configuration/raw?only_validate=true'
```

### Stick table

See entries for stick table called `main`:

```console
$ curl -i -u admin:admin 'http://localhost:30555/v3/services/haproxy/runtime/stick_tables/main/entries'
```

## Deploy a FaaS function with custom timeout

```console
$ faas-cli store deploy figlet --env exec_timeout=8s
```

Note here we only set `exec_timeout`, while `read_timeout` and `write_timeout`
use the default gateway values.

How to set the upstream timeout in OpenFaaS CE Gateway:

```console
$ sudo kubectl edit deployment gateway
...modify "upstream_timeout" to X seconds, then save and quit
$ sudo kubectl get pods
...see that the gateway pod has restarted with the new timeout
```
