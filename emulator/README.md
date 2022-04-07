# Emulator
We emulate edge scenarios with multiple nodes by relying on [Containernet](https://containernet.github.io/).

## Setup

### Requirements
- Ubuntu 20.04 (recommended)
- Python3
- Docker CE 20.10.14
- Docker Compose v2
- Sysbox CE 0.5.0

For Docker CE, Docker Compose and Sysbox CE you can look at the [README in the project root.](../README.md).

### Install Containernet v3.1
```shell
sudo apt-get install ansible git
git clone --branch v3.1 https://github.com/containernet/containernet.git
#  We opened a pull request (#243) to make this edit available directly from upstream. See the PR for further details.
cp hack/node.py containernet/mininet/node.py
sudo ansible-playbook -i "localhost," -c local containernet/ansible/install.yml
# Not needed if Docker was previously installed and setup properly
sudo usermod -aG docker $USER
```

### Install other Python packages
```shell
sudo pip3 install -r requirements.txt
```

## Examples
See [example.py](example.py)