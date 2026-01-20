# Ansible playbooks for VMs configuration

This folder contains a set of Ansible playbooks that can be used to speed up the
configuration of the four VMs used for data collection. Unfortunately, the
playbooks are not complete and some manual intervention is required to configure
the VMs correctly.

The VMs are listed below (you will almost certainly need to change the IP
addresses):

| VM name                | IP         |
|------------------------|------------|
| traffic-dfaas-operator | 10.12.38.3 |
| traffic-dfaas-mid      | 10.12.38.4 |
| traffic-dfaas-light    | 10.12.38.9 |
| traffic-dfaas-heavy    | 10.12.38.2 |

Run the provided Ansible playbooks in the correct order to first set up all FaaS
nodes:

```console
$ ansible-playbook --inventory inventory.yaml 01-init.yaml
$ ansible-playbook --inventory inventory.yaml 02-minikube.yaml
...
```

Make sure to configure the nodes (IP addresses, passwords, etc.) in the
dedicated YAML files under the `host_vars` and `group_vars` directories, or
overwrite the [`inventory.yaml`](inventory.yaml) file. Make sure also to update
the IP in the [`kubeconfig`](kubeconfig) file!

If Ansible is not installed, you can install it using `pip` or refer to the
[official
documentation](https://docs.ansible.com/projects/ansible/latest/installation_guide/installation_distros.html)
for instructions on specific operating system. We assume Ubuntu 24.04 is used.

> [!WARNING]
> After execuring the `02-minikube.yaml` playbook make sure to reboot the VM to
> let user `user` run Docker commands without root permissions. See [Docker
> official
> docs](https://docs.docker.com/engine/install/linux-postinstall/#manage-docker-as-a-non-root-user)
> for more information.

> [!IMPORTANT]
> The Minikube instance is not automatically started by the playbooks. You must
> manually run the [`minikube_builder.sh`](../minikube_builder.sh) Bash script
> on the remote machines. It is recommended to copy the entire repository to
> each machine before running the script. The long-term goal is to remove this
> script, but for now it is still required.

After running the `minikube_builder.sh` script, you can execute the final
playbook (`06-iptables.yaml`) to properly configure the iptables rules and allow
the Kubernetes cluster to be reached from the operator VM.

> [!TIP]
> You can run the Ansible playbooks from any machine that has Ansible installed
> and network connectivity to the four VMs. We recommend running them directly
> from `traffic-dfaas-operator`, although this is not required.

From this point onward, this document contains original notes created by the
thesis student who developed the Bash script and performed the initial VM setup.

## Original document

Author: Thomas Howard-Grubb
Edited by: Emanuele Petriglia

Todo with Ansible:

* Start the Minikube node on each FaaS node:

    minikube start --memory=max --cpus=max --apiserver-ips='10.12.38.4'

* Start OpenFaaS CE:

    arkade install openfaas-ce

* Configure faas-cli:

    kubectl rollout status -n openfaas deploy/gateway
    kubectl port-forward -n openfaas svc/gateway 8080:8080 &

    # If basic auth is enabled, you can now log into your gateway:
    PASSWORD=$(kubectl get secret -n openfaas basic-auth -o jsonpath="{.data.basic-auth-password}" | base64 --decode; echo)
    echo -n $PASSWORD | faas-cli login --username admin --password-stdin

    faas-cli list

* Apply custom YAML resources (without Scaphandre):

    kubectl apply -f dfaas-thomas/metrics_predictions/infrastructure/

* Restart Prometheus (to read the new config):

    kubectl rollout restart deployment prometheus -n openfaas

* Copy custom script inside Minikube node:

    minikube cp dfaas-thomas/metrics_predictions/samples_generator/find-pid.py /etc/

* Deploy OpenFaaS functions (for each FaaS node):

    faas-cli store deploy figlet


* Software to install on operator VM: `vegeta`, `jq`, `kubectl`, `faas-cli`.


## Node setup

Run the given Ansible playbooks in the right order to first setup all FaaS
nodes:

```console
$ ansible-playbook --inventory inventory.yaml 01-init.yaml
$ ansible-playbook --inventory inventory.yaml 02-minikube.yaml
...
```

Make sure to configure the nodes (IP, password...) in the
[`inventory.yaml`](inventory.yaml) file!

If you do not have Ansible installed, use `pip` or see the [official
documentation](https://docs.ansible.com/projects/ansible/latest/installation_guide/installation_distros.html)
for specific operating systems.

### Minikube instance

Minikube start command:

```console
$ minikube start --memory=max --cpus=max --apiserver-ips=IP
```

The first two arguments guarantee that Minikube uses all available resources.
The last argument allow the Minikube instance to accept incoming connections
to the Kubernetes cluster from other hosts to the host where Minikube runs.

### External connectivity

You need to add three rules with iptables for each service that need to be
accessed from outside: OpenFaaS (port 30411), Prometheus (port 9090) and
Kubernetes API (port 8443). You can use the
[`03-iptables.yaml`](03-iptables.yaml) playbook to automatically add the rules
to all nodes, or you can manually add them. Just make sure to change the node'IP
and the port:

```console
$ sudo iptables -t nat -A PREROUTING -p tcp -d NODE_IP --dport SVC_PORT -j DNAT --to-destination 192.168.49.2:SVC_PORT
$ sudo iptables -t nat -A POSTROUTING -p tcp -d 192.168.49.2 --dport 30411 -j SNAT --to-source NODE_IP
$ sudo iptables -I FORWARD 1 -p tcp -d 192.168.49.2 --dport SVC_PORT -j ACCEPT
```

In brief:

* The first rule redirects incoming TCP packets destined for port `SVC_PORT` on
  the node's external IP `NODE_IP` to the Minikube node's internal IP (usually
  it is always 192.168.49.2.
* The second rules ensures that return packets from Minikube to external clients
  use the node's IP as the source address, to avoid asymmetric routing problems.
* THe third rule explicitly allows forwarding of TCP packets to the Minikube
  node on port `SVC_PORT`, avoiding eventual restrictive policies for incoming
  packets.
