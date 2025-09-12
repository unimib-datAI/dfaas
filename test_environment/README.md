# Multiple VMs Environment

> [!WARNING]  
> This testing environment is not supported and won't work with latest DFaaS
> versions! We are rewriting this environment to support Kubernetes.

This environment has been exploited to execute some comparison tests beetween the old load balancing strategy adopted by the DFaaS Agent, and the newly implemented Node Margin Strategy.  
It consists of three DFaaS nodes, called "Node Light" (2 CPU, 8 GB RAM), "Node Mid" (4 CPU, 16 GB RAM) and "Node Heavy" (6 CPU, 24 GB RAM), and an "Operator" node which automatically deploys functions on other nodes and starts load tests. The technical specifications of the three DFaaS nodes correspond to the specifications of the three nodes types on which the predictive models exploited by the Node Margin Strategy have been trained.  
To deploy this environment you need three VMs with the specifications reported above, and antoher VM for the [Operator](../operator).

## Setup and deploy the environment
Install [Ansible](https://www.ansible.com/), an agentless automation tool that you install on a single host, referred to as the control node.  
Then, using the [setup_playbook.yaml](setup_playbook.yaml) file, your Ansible control node can setup the environment to execute DFaaS on the managed node(s) specified in an inventory file.

Run the `ansible-playbook` command on the control node using an inventory file configured like [inventory_example.yaml](inventory_example.yaml), to execute the tasks specified in the playbook with the following options:

`-i` : path to an inventory file  
`--extra-vars` : to specify the Sysbox version and shiftfs branch to be installed  
`--tags` : to specify steps of the playbook to be executed

> The following command assumes you are using Ubuntu 20.04 LTS with kernel version 5.4.

```shell
ansible-playbook -i inventory.yaml setup_playbook.yaml --extra-vars "sysbox_ver=0.5.2 shiftfs_ver=k5.4" --tags "installation, deploy, start"
```

Tags have the following meaning:
- `installation`: install required software
- `deploy`: copy files and build Docker images of DFaaS nodes on VMs
- `start`: run DFaaS nodes containers
- `deploy-operator`: copy files and build Docker image on Operator node VM
- `start-operator`: run DFaaS Operator container
- `stop`: stop DFaaS nodes running containers
- `leave-swarm`: each VM leaves the Docker Swarm cluster
- `remove`: delete from VMs DFaaS directory and Docker images
- `remove-operator`: delete from Operator node VM the operator directory and Docker image

# Executed tests
The results contained in [tests_results](tests_results) directory are referred to the executed experiments described below.

#### Node Margin Strategy thresholds
Node Light:
- CPU: 104%
- RAM: 4.1 GB
- Power: 0.7 W  

Node Mid:
- CPU: 190%
- RAM: 5.5 GB
- Power: 2.1 W  

Node Heavy:
- CPU: 235%
- RAM: 6.0 GB
- Power: 3.5 W  

## First test
- `Duration`: 5 minutes
- `Load`: 250 req/s to _figlet_ function on Node Light

#### Static Strategy maxrates
_figlet_:
- Node Light: 200 req/s
- Node Mid: 450 req/s
- Node Heavy: 700 req/s

## Second test
- `Duration`: 10 minutes
### Phase 1 (minutes 0-5)
- `Load`: 100 req/s to _figlet_ function on Node Mid; 50 req/s to _shasum_ function on Node Mid
### Phase 2 (minutes 5-10)
- `Load`: 300 req/s to _figlet_ function on Node Mid; 50 req/s to _shasum_ function on Node Mid

#### Static Strategy maxrates
_figlet_:
- Node Light: 100 req/s
- Node Mid: 225 req/s
- Node Heavy: 350 req/s  

_shasum_:
- Node Light: 100 req/s
- Node Mid: 225 req/s
- Node Heavy: 350 req/s

## Third test
- `Duration`: 15 minutes
### Phase 1 (minutes 0-5)
- `Load`: 350 req/s to _figlet_ function on Node Light
### Phase 2 (minutes 5-10)
- `Load`: 350 req/s to _figlet_ function on Node Light; 100 req/s to _figlet_ function on Node Mid; 100 req/s to _figlet_ function on Node Heavy
### Phase 3 (minutes 10-15)
- `Load`: 350 req/s to _figlet_ function on Node Light; 600 req/s to _figlet_ function on Node Mid; 100 req/s to _figlet_ function on Node Heavy

#### Static Strategy maxrates
_figlet_:
- Node Light: 200 req/s
- Node Mid: 450 req/s
- Node Heavy: 700 req/s
