# OpenWhisk

[Apache OpenWhisk](https://openwhisk.apache.org/) is an open source, distributed
serverless platform that executes functions in response to events at any scale.

This directory contains an Helm chart that can be used to deploy the core
OpenWhisk platform to a single-node Kubernetes cluster. This is a fork of the
original OpenWhisk Helm chart, created at commit
[146d249](https://github.com/apache/openwhisk-deploy-kube/commit/146d24925564e2871f4cc6506d4e92098968457d).
We maintain a custom chart because the original chart is no longer maintained
and includes many components that we either do not use or provide through
alternative solutions.

## Chart details

The chart is tailored to a minimal single-node deployment: every component runs
as a single replica, user actions run as Kubernetes pods
(KubernetesContainerFactory), and OpenWhisk is exposed over plain HTTP through a
NodePort.

It creates the following Kubernetes resources:

* nginx the OpenWhisk REST API, exposed externally as a NodePort on port 31001
  (`whisk.apiHostPort`).

* Internal services: apigateway, controller, couchdb, kafka, redis, user-events,
  and zookeeper.

* Control plane pods:
   * Deployments: apigateway, couchdb, nginx, redis, and user-events.
   * StatefulSets: controller, invoker, kafka, and zookeeper.
   * Pod: wskadmin (run `kubectl exec -it <my-release>-wskadmin -- wskadmin ...`)

* Jobs: init-couchdb (wipes and initializes the database) and install-packages
  (routemgmt + catalog)

* Persistent volume claims: couchdb-pvc, kafka-pvc, redis-pvc,
  zookeeper-pvc-data, and zookeeper-pvc-datalog.

Note that in this deployment, the invoker creates and removes pods for actions
as the load changes.

## Installing

Assuming the chart has been downloaded to the DFaaS repository in your home
directory, you can install the OpenWhisk chart using the Helm CLI:

```bash
$ helm install openwhisk ./dfaas/k8s/charts/openwhisk --set whisk.apiHostName=<node-ip-address>
```

This will deploy OpenWhisk with the release name `openwhisk` in the `default`
namespace. Note that you must set the node's IP address manually.

Once the `openwhisk-install-packages` POD is in the Completed state, you can
configure the OpenWhisk CLI utility and start using the platform:

```bash
$ Set wsk CLI
$ wsk property set --apihost http://10.12.38.16:31001
$ wsk property set --auth "<uuid>:<secret>"
$ # Test wsk CLI
$ wsk property get
$ wsk list --verbose
```

Since we are not using OpenWhisk in production, you can simply use the fixed
administrator credentials:
`789c46b1-71f6-4ed5-8c54-816aa4f8c502:abczO3xZCLrMN6v2BKK1dXYFpXlPkccOFqm12CdAsMgRU4VrNZ9lyGVCGuMDGIwP`.

### Configuration

The [values.yaml](./values.yaml) file contains the few supported options: the
external host/port (`whisk.apiHostName`, `whisk.apiHostPort`), the OpenWhisk
version (`whisk.gitTag`), the credentials (`whisk.auth`, `db.auth`), the CouchDB
database prefix, the cluster domain and the StorageClass. Everything else is set
directly in the templates.

## License

Copyright © 2026 The DFaaS Authors.

The source code in this Helm chart is licensed under the Apache License, Version
2.0 (the "License"). You may obtain a copy of the License at
<https://www.apache.org/licenses/LICENSE-2.0>.

The complete list of DFaaS authors and contributors can be found in the
[AUTHORS](../../../AUTHORS) file or on the [DFaaS GitHub contributors
page](https://github.com/unimib-datAI/dfaas/graphs/contributors).

This work is derived from [Apache OpenWhisk Deploy
Kubernetes](https://github.com/apache/openwhisk-deploy-kube), originally
developed by contributors to the Apache OpenWhisk project. The original work is
also licensed under the Apache License, Version 2.0.

The following notice is retained from the original project:

> Licensed to the Apache Software Foundation (ASF) under one or more contributor license agreements. See the NOTICE file distributed with this work for additional information regarding copyright ownership.
>
> The ASF licenses this file to You under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at
>
> <https://www.apache.org/licenses/LICENSE-2.0>
>
> Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
