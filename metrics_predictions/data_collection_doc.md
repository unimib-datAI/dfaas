# Data Collection

This section will be dedicated to the data collection process, describing
infrastructure with choices made and result structure.

## Infrastructure

The data collection process utilizes the code originally developed by an
un-cited source which was adapted to operate on a different infrastructure. Four
virtual machines were employed for the data collection, all connected through
the same VPN:

* One traffic generator node.
* Three DFaaS nodes, each provisioned with distinct hardware configurations.

The specifications of the virtual machines are summarized in the table below.

## DFaaS Nodes VMs

The role of the three DFaaS VMs was to simulate three nodes of the DFaaS system.
To achieve this, multiple steps were taken to ensure the nodes not only behaved
like DFaaS nodes but were also accessible to the generator node running the data
gathering.

**Building the Node**

The first step was running minikube:

```bash
minikube start --memory max --cpus max --apiserver-ips=ip_of_the_node
```

* `-memory max` and `-cpus max` guarantee that the minikube instances are
  allowed to use all the available computing resources of the host machine.
* The `-apiserver-ips` option is a key element in the setup.

By default, Minikube is designed to facilitate communication between the host
machine and the Kubernetes cluster running inside the Minikube virtual
environment. However, in this default configuration, any packets sent from the
host to Minikube will carry the host's local IP address as the source. When
other machines on the network need to interact with the Kubernetes API server,
this local address is not enough.

The `apiserver-ips` flag allows Minikube to advertise and accept connections
directed to the actual network IP address of the host machine, ensuring that
packets originating from outside are correctly accepted and routed by the API
server. Once the minikube cluster is running the application is built following
the steps described.

| VM Name | Specifications |
| :--- | :--- |
| **VM Traffic Generation THG** | 4 vCPU, 16 GB RAM, 20 GB Disk<br>IP Address: 10.99.150.242 |
| **Light node THG** | 2 vCPU, 8 GB RAM, 20 GB Disk<br>IP Address: 10.99.174.226 |
| **Mid node THG** | 4 vCPU, 16 GB RAM, 20 GB Disk<br>IP Address: 10.99.217.210 |
| **Heavy node THG** | 6 vCPU, 24 GB RAM, 20 GB Disk<br>IP Address: 10.99.241.50 |

### Setting up External Connectivity

An essential step in ensuring the correct functioning of the data collection
process was making the services deployed inside Minikube reachable to external
machines. To achieve a permanent and reliable solution for this external
connectivity requirement, `iptables` was configured to forward incoming traffic
to the appropriate internal Minikube addresses.

Specifically, OpenFaaS, Prometheus, and the Kubectl API were the services that
needed to be accessible from outside the hosting virtual machine. OpenFaas was
exposed to enable remote management and deployment of serverless functions,
while Prometheus needed to be reachable in order to allow resource usage metrics
to be queried during function execution.

Both services were already accessible on specific ports from the hosting VM, but
to allow access from external machines, a set of `iptables` rules was added to
forward traffic appropriately.

The following rules were applied:

```bash
$ sudo iptables -t nat -A PREROUTING -p tcp -d 18.99.217.210 --dport 30411 -j DNAT --to-destination 192.168.49.2:30411
$ sudo iptables -t nat -A POSTROUTING -p tcp -d 192.168.49.2 --dport 30411 -j SNAT --to-source 10.99.217.210
$ sudo iptables -I FORWARD 1 -p tcp -d 192.168.49.2 --dport 30411 -j ACCEPT
```

These rules handle:

* **PREROUTING:** Redirects incoming TCP packets destined for port 30411 on the
  VM's external IP (10.99.217.210) to the Minikube node's internal address
  (192.168.49.2:30411).
* **POSTROUTING:** Ensures that return packets from Minikube to external clients
  use the VM's IP (10.99.217.210) as the source address, avoiding asymmetric
  routing problems.
* **FORWARD:** Explicitly allows forwarding of TCP packets to the Minikube node
  on port 30411, ensuring that restrictive forwarding policies are not applied
  to incoming packets.

The rules shown above correspond to the configuration applied on the Nodo Mid
THG virtual machine for forwarding traffic on port 30411 (used by OpenFaaS).
However, the same rules were applied not only for port 30411 (OpenFaaS), but
also for port 9090 (Prometheus) and port 8443 (Kubernetes API server for kubectl
access). Additionally, this forwarding setup was replicated on each of the node
VMs in the deployment, ensuring that the relevant services on all nodes were
reachable from external machines.

### Traffic Generator VM

The first challenge was configuring `kubectl` to manage all three Minikube
clusters remotely and simultaneously. To address this, a custom `kubectl`
configuration file was created. This file defines a separate context for each
node, associating each one with its corresponding IP address. Once configured,
switching between nodes could be achieved by using the
`context=<node-context-name>` flag in any `kubectl` command.

The `kubectl` custom config file simply defines 3 different `kubectl` contexts,
one for each machine. These are taken from each machine. The repository is made
to work for the mid node; IP addresses and `kubectl` context were hardcoded. A
better approach would be to use environment variables to make the setup of the
VMs easier. The hardcoded values need to be changed in all `.py` and bash files
in the sample-generator directory based on the details of the new machine. In
particular: IP addresses, `kubectl` context, and `vegeta` commands.

Three scripts were created to automate the creation of the nodes:

1. `minikube_builder.sh`: Needed on the individual nodes to build the minikube
   instance.
2. `gathering_controller.sh`: Starts the data gathering (hard coded sample
   generator invocation; can be changed inside the file).
3. `restart_gathering.sh`: Restarts the data gathering when scripts stall.

It is important to make sure that all of them are executable: the first on the
node VMs, the other two on the generator node. It is also important to transfer
the ssh keys to the new generator VM (example for one of the nodes: `ssh-copy-id
thomashg@10.99.217.210`).

The execution of the samples generator for each machine was done through screen
commands, one for each machine. To run the data gathering, run the
`gathering_controller.sh` command.
