# Introduction

> [!WARNING]
> This section is under active development. We are currently improving the
> metrics collection process based on profiling of FaaS functions.

The work conducted allows for gathering system metrics from FaaS functions and nodes based on the input load per second received by the functions. The data collected in this manner will then be used to model the performance of a node. The components and their interactions can be seen in the simplified figure.

![Figure Title](/images/workflow.png)

## Infrastructure for Sampler Generator

To run the Sampler Generator with the correct infrastructure, a comprehensive guide is provided below. It is assumed that the components are being executed on a Virtual Machine created using [Qemu](https://www.qemu.org/) technology in collaboration with KVM kernel components. Virtualization makes it possible to collect data from various types of nodes, but it adds additional steps to achieve a fully functional infrastructure.

To utilize the various components, start by [creating a virtual machine on your host](https://ubuntu.com/server/docs/virtualization-qemu). At the end of the creation of a VM, select customize configuration before install, then go on display and on the first option select VNC instead of Spice. Within this virtual machine, set up a Kubernetes cluster using Minikube. You can follow the guide at the link: [Minikube Setup Guide](https://minikube.sigs.k8s.io/docs/start).

Next, you need to deploy the OpenFaaS cluster within Minikube using the following guide: [OpenFaaS Kubernetes Deployment Guide](https://docs.openfaas.com/deployment/kubernetes).

Subsequently, the exporters "Node Exporter" and "cAdvisor" can be deployed in the cluster using the following commands from the 'metrics_predictions/infrastructure' folder:
 
```shell
kubectl apply -f cadvisor-daemonset.yaml
kubectl apply -f cadvisor-service.yaml
kubectl apply -f node-exporter-deamonset.yaml
kubectl apply -f node-exporter-service.yaml
kubectl apply -f nodeport-service.yaml
kubectl apply -f prometheus-config.yaml
```
Next, take the Scaphandre executable present in the 'metrics_predictions/infrastructure/scaphandre_image' folder and execute it on the host using the command:

```shell
nohup sudo ./scaphandre qemu &
```
Now it's possible to propagate the data related to VM consumption by completing the steps described in the [Scaphandre Deployment Guide](https://hubblo-org.github.io/scaphandre-documentation/). Then, execute the following command from the 'metrics_predictions/infrastructure' directory.

```shell
helm install scaphandre helm/scaphandre
```

Finally, run the command to propagate the energy metrics to Kubernetes:
```shell
minikube mount /var/scaphandre:/var/scaphandre
```

To access these Exporters through Prometheus, apply the following command from the 'metrics_predictions/infrastructure' directory:
```shell
kubectl delete -f prometheus-config.yaml
```

As a final step, insert the file 'metrics_predictions/samplers_generator/find-pid.py' into Minikube using the command:
```shell
docker cp ./samples_generator/find-pid.py minikube:/etc/
```

## Sampler Generator

It is responsible for generating the different load configurations to be sent to the functions deployed on OpenFaaS, querying the Prometheus metrics collector to collect data and metrics regarding the state of the node and functions and empirically evaluate the state of the system. At the end of the process, the output of the Sampler Generator consists of a number of variable number of files that will be used by the System Forecaster as a database. To execute the Sampler Generator:
```shell
python3 samples-generator.py 50 20s 
```
*Note: To modify the FaaS to be deployed, edit the file `samples-generator.py`.*

The tool will start from the configuration specified in the metrics_predictions/samples_generator/configuration.txt file.

Output files can be found in the directory 'metrics_predictions/output/output-energy/.

## Sampler Generator Profiler

It's a component very similar to the Sampler Generator, which takes a list of functions as input and is responsible for collecting data for each function individually by deploying them one by one on the node and saving the data into separate files.

Output files can be found in the directory 'metrics_predictions/output/generator/.

## Function Profiler

This component is capable of grouping functions into clusters based on their consumption profiles. It accomplishes this task using the data collected through the Sampler Generator Profiler.

Output file can be found in 'metrics_predictions/group_list.json'.

## System Forecaster

This module is responsible for predicting system metrics using as input the requests per second received by each function present on a node. It preprocesses the data by removing outliers, normalizing the data, and grouping columns of metrics belonging to functions with a similar consumption profile. Subsequently, it proceeds to train the machine learning models.

Output files can be found in the 'metrics_predictions/system-forecaster-models' directory.

## Predict Profile

It's a component that, given Sampler Generator Profiler input data for a new function and the previously created model from the Function Profiler, can assign the new function to an existing cluster. This allows obtaining system metrics for this new function without needing to retrain the System Forecaster models and without collecting excessive data for this function.

The component modifies the 'metrics_predictions/group_list.json' file.
