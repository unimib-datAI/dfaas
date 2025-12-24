# Data Collection

> [!IMPORTANT]
> Work in progress section!

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
