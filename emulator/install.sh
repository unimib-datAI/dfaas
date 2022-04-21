#!/bin/bash

set -e

sudo apt-get install -yy ansible git

# This fork contains unmerged PRs to upstream that provide a successful installation
# git clone --branch v3.1 https://github.com/containernet/containernet.git
git clone https://github.com/ElectricalBoy/containernet.git

#  We opened a pull request (#243) to make this edit available directly from upstream. See the PR for further details.
cp hack/node.py containernet/mininet/node.py
cp hack/install.yml containernet/ansible/install.yml

sudo ansible-playbook -i "localhost," -c local containernet/ansible/install.yml -e 'ansible_python_interpreter=/usr/bin/python3'

sudo pip3 install -r requirements.txt