# Commands

This is a short list of useful commands for a development environment. Warning:
this section is still a work in progress.

## Deploy the entire DFaaS node

The [deploy.just](../deploy.just) recipes deploy DFaaS on the local k3s node,
which must be already set up (see the
[prerequisites](../README.md#deploy-with-just)). There are three deployment
modes:

* `just deploy-all CONFIG`: deploys the Agent, HAProxy, rejector, Prometheus,
  FaaS platform and functions,
* `just deploy-no-agent`: deploys all but the Agent,
* `just deploy-faas`: deploys only the FaaS platform, functions and Prometheus,
  which both platforms require.

The functions are the ones in the [functions](../functions) directory: `mlimage`,
`memstress` and `noop`. All the recipes can be run again to update an existing
deployment. There is also a recipe for each component, see `just --list`.

`CONFIG` is the Helm values file of the agent, it overrides the [chart's default
values](../k8s/charts/agent/values.yaml). See
[agent-startup.md](agent-startup.md) for the available options. By default the
chart uses local images, so you need to build them first:

```console
$ just agent k3s
$ just forecaster k3s
$ just deploy-all CONFIG.yaml
```

Without the agent, HAProxy replies to function calls with `503`, until you
configure it. With `just deploy-faas` there is no HAProxy, so call the functions
directly on the platform's port: `31112` for OpenFaaS and `31001` for OpenWhisk.

The default FaaS platform is OpenFaaS. To deploy OpenWhisk, set the `platform`
variable:

```console
$ just platform=openwhisk deploy-faas
```

The agent must be configured for the same platform, otherwise `just deploy-all`
stops before changing anything. An example of `CONFIG.yaml` for OpenWhisk:

```yaml
config:
  AGENT_FAAS_PLATFORM: "openwhisk"
  AGENT_FAAS_HOST: "openwhisk-nginx.default.svc.cluster.local"
  AGENT_FAAS_PORT: 80
  AGENT_OPENWHISK_NAMESPACE: "whisk.system"
  AGENT_OPENWHISK_API_KEY: "<the administrator credentials in k8s/charts/openwhisk/values.yaml>"
```

The node IP address for OpenWhisk is detected automatically. If it is wrong, set
it with `just platform=openwhisk node_ip=IP deploy-faas`.

The OpenFaaS functions share the same timeouts, which you can change with the
variables at the top of [deploy.just](../deploy.just). With the defaults,
`memstress` must be called with `seconds` below 4. See [timeouts.md](timeouts.md)
for more information:

```console
$ just function_replica_timeout=60s function_gateway_timeout=61s function_proxy_timeout_ms=62000 deploy-faas
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
$ faas-cli store deploy figlet --env exec_timeout=4s --env max_inflight=800 --label dfaas.timeout_ms=6000
```

Or with a custom function not in store:

```console
$ faas-cli deploy --image=ghcr.io/unimib-datai/dfaas-openfaas-image-classification:dev --name=mlimage --env exec_timeout=4s --env max_inflight=400 --label dfaas.timeout_ms=6000
```

See [timeouts.md](timeouts.md) file for information about these env vars, while
[agent-strategies](agent-strategies.md) for the label.

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
