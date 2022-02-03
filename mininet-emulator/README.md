# Mininet Emulator

## Setup

### Requirements
- Ubuntu 20.04
- Python3

### Instructions

**Install Containernet**
```shell
sudo apt-get install ansible
git clone https://github.com/containernet/containernet.git
sudo ansible-playbook -i "localhost," -c local containernet/ansible/install.yml
sudo usermod -aG docker $USER
```

**Build Docker images**
```shell
cd agent && docker build -t=agent:latest --file=Dockerfile ../../dfaasagent && cd ..
cd proxy && docker build -t=proxy:latest . && cd ..
```

### VirtualBox Hacks

```shell
sudo usermod -aG vboxsf $USER # Allows to mount rootless host shared folders
```