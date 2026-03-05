# Commands

This is a short list of useful commands for a development environment. Warning:
this section is still a work in progress.

## Deploy the entire DFaaS node

Start by deploying DFaaS dependencies:

```console
$ sudo helm install haproxy haproxytech/haproxy --values k8s/charts/values-haproxy.yaml --version 1.26.1
$ sudo helm install prometheus prometheus-community/prometheus --values k8s/charts/values-prometheus.yaml --version 27.37.0
$ sudo helm install openfaas openfaas/openfaas --values k8s/charts/values-openfaas.yaml --version 14.2.128
```

If you are running a debugging node, run also:

```console
$ sudo kubectl apply -f k8s/scripts/prometheus-nodeport-service.yaml
```

Next deploy the custom stick-table-exporter service:

```console
$ ./k8s/scripts/build-image.sh stick-table-exporter k3s --dockerfile k8s/scripts/stick-table-exporter/Containerfile
$ sudo helm install stick-table-exporter k8s/scripts/stick-table-exporter/helm/
```

Finally deploy the DFaaS agent with a custom configuration:

```console
$ ./k8s/scripts/build-image.sh agent k3s
$ sudo helm install agent ./k8s/charts/agent/ --values CONFIG.yaml
```

The DFaaS node is now ready, but with no function deployed.

## Deploy with OpenWhisk instead of OpenFaaS

```console
$ sudo helm install owdev owdev/openwhisk --values k8s/charts/values-openwhisk.yaml
$ sudo helm install haproxy haproxytech/haproxy --values k8s/charts/values-haproxy.yaml --version 1.26.1
$ sudo helm install prometheus prometheus-community/prometheus \
    --values k8s/charts/values-prometheus.yaml \
    --values k8s/charts/values-prometheus-openwhisk.yaml \
    --version 27.37.0
$ sudo helm install agent ./k8s/charts/agent/ --values values-test.yaml \
    --set config.AGENT_FAAS_PLATFORM=openwhisk \
    --set config.AGENT_OPENFAAS_HOST=owdev-nginx.openwhisk \
    --set config.AGENT_OPENFAAS_PORT=80 \
    --set config.AGENT_OPENWHISK_NAMESPACE=guest \
    --set config.AGENT_OPENWHISK_API_KEY="<uuid>:<key>"
```

To retrieve the OpenWhisk API key after deployment:

```console
$ kubectl -n openwhisk exec deploy/owdev-wskadmin -- wskadmin user get guest
```

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

## Deploy a FaaS function with custom env vars

```console
$ faas-cli store deploy figlet --env exec_timeout=4s --env max_inflight=800
```

See [timeouts.md](timeouts.md) file for information about these env vars.

You can also change the env vars with `sudo kubectl edit deployment figlet`. The
pods will be restarted!

Test the deployed function with `curl`:

```console
$ curl http://localhost:30080/function/figlet -d "Hello World!" -i
```

How to set the upstream timeout in OpenFaaS CE Gateway:

```console
$ sudo kubectl edit deployment gateway
# ...modify "upstream_timeout" to X seconds, then save and quit
$ sudo kubectl get pods
# ...see that the gateway pod has restarted with the new timeout
```
