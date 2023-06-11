## Introduction
The work, carried out in an isolated environment, allows simulating different load scenarios where certain functions, released in the cluster as FaaS, are subjected to an incremental number of requests per unit time. During the load tests, metrics are collected, processed and eventually used as a database to train Machine Learning models that are able to model the state of the system.

It is possible to divide the work produced into three macro-phases:
1. Load testing: In this first phase, different load profiles are generated. Different load conditions on different subsets of FaaS functions are simulated. The confi-
gurations are generated based on the input consisting of: list of functions to be tested on, maximum number of requests per second to be sent, and duration of the test;
2. Data collection: Each test episode corresponds to a data collection phase. In this phase, metrics regarding the state of the physical machine, the state of the functions and other information related to the context of the execution are collected. Based on the collected data, the state of the system is empirically evaluated;
3. Prediction: In this phase, after collecting data from all load tests, Machine Learning models are trained that are able to make predictions about the state of the system when subjected to certain conditions.

## Infrastructure
First of all, you have to set up a Kubernetes cluster. If you are working on a local machine you can leverage on **Minikube**, that allows to run a single-node Kubernetes cluster on your own local machine.
To install the cluster: `minikube start`.

After that, you have to install the FaaS platform. For this work we will be using **OpenFaaS** (on top of Kubernetes). OpenFaaS brings with it the metric collector **Prometheus** which it will be interrogated to scrape the needed metrics.
To install OpenFaaS it is recommended to follow this guide: https://docs.openfaas.com/deployment/kubernetes/.

Prometheus alone is not sufficient and it is therefore necessary to deploy 2 exporters: Node Exporter and cAdvisor to scrape from the node and containers running on it, respectively. The files for the deployment instructions can be found in the `infrastructure` folder.

## Samples Generator
It is responsible for generating the different load configurations to be sent to the functions deployed on OpenFaaS, querying the Prometheus metrics collector to collect data and metrics regarding the state of the node and functions and empirically evaluate the state of the system. At the end of the process, the output of the Sampler Generator consists of a number of variable number of files that will be used by the System Forecaster as a database.
To execute the Samples Generator: `python3 samples-generator.py 50 20s` (It may take a lot to be completly executed).
n.b. To modify the FaaS to be deployed edit the file `samples-generator`.

## System Forecaster
This module receives as input one or more files that are used as a database to train Machine Learning models to predict the state of the system under certain loading conditions. Briefly, the module performs a preliminary data preprocessing phase, a training phase, and finally a phase to evaluate the quality of the prediction of the model. The System Forecaster performs three types of Machine Learning tasks:
- Multi-output regression task: to predict the CPU and RAM values of the node;
- Multi-output classification task: to predict the discretized values of CPU and RAM of the node;
- Classification task: to predict whether the system is in an overloaded state or not.