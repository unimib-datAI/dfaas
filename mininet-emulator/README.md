# Mininet Emulator

## Setup

### Requirements
- Ubuntu 20.04
- Python3

### Instructions

**Install Docker 20.10.12**
```shell
# TODO
```

**Install sysbox-ce**
```shell
# TODO
```

**Install Containernet**
```shell
sudo apt-get install ansible git
git clone https://github.com/containernet/containernet.git
cp hack/containernet/mininet/node.py containernet/mininet/node.py
sudo ansible-playbook -i "localhost," -c local containernet/ansible/install.yml
sudo usermod -aG docker $USER
```

**Build Docker images**
```shell
docker build -t dfaas-agent-builder:latest -f docker/dfaas-agent-builder.dockerfile ../dfaasagent
docker build -t dfaas-node:latest -f docker/dfaas-node.dockerfile ./docker
```

** Deploy a function in a node**
```shell
faas-cli login --password admin && faas-cli store deploy figlet --label dfaas.maxrate=10
```