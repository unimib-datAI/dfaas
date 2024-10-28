# Multiple VMs Environment
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