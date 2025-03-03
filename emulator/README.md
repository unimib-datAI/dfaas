# Emulator

We emulate edge scenarios with multiple nodes by using [Containernet](https://containernet.github.io/).

## Setup

### Requirements

- Ubuntu 20.04 (recommended)
- Python3
- Docker CE 20.10.14
- Docker Compose v2
- Sysbox CE 0.5.0

For Docker CE, Docker Compose and Sysbox CE you can look at the [README in the project root.](../README.md).

### Installation methods

#### Install using the convenience script

```shell
./install.sh
```

#### Manual

_Install Containernet v3.1_

```shell
sudo apt-get install ansible git
git clone --branch v3.1 https://github.com/containernet/containernet.git
#  We opened a pull request (#243) to make this edit available directly from upstream. See the PR for further details.
cp hack/node.py containernet/mininet/node.py
cp hack/install.yml containernet/ansible/install.yml
sudo ansible-playbook -i "localhost," -c local containernet/ansible/install.yml
```

_Install Python packages_
```shell
sudo pip3 install -r requirements.txt
```

Files in the [`hack`](hack) directory are required to enable runtime selection
with containernet, currently not supported. See the associated pull request in
[containernet/containernet](https://github.com/containernet/containernet/pull/243)
for more information.

## Examples

See [example.py](example.py)
