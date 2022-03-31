# Emulator
We emulate edge scenarios with multiple nodes by relying on [Containernet](https://containernet.github.io/).

## Setup

### Requirements
- Ubuntu 20.04 (recommended)
- Python3
- Docker CE
- Sysbox CE

For Docker and Sysbox you can look at the [README in the project root.](../README.md).

### Install Containernet
```shell
sudo apt-get install ansible git
git clone https://github.com/containernet/containernet.git
#  We opened a pull request (#243) to make this edit available directly from upstream. See the PR for further details.
cp hack/containernet/mininet/node.py containernet/mininet/node.py
sudo ansible-playbook -i "localhost," -c local containernet/ansible/install.yml
# Not needed if Docker was previously installed and setup properly
sudo usermod -aG docker $USER
```

## Examples

> TODO