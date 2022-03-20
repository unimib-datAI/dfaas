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
docker build -t dfaas-agent-builder:latest -f docker/dfaas-agent-builder.dockerfile ../
docker build -t dfaas-node:latest -f docker/dfaas-node.dockerfile ./docker
```
