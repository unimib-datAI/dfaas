#!/bin/bash

set -e

sudo apt-get install -yy ansible git

git clone --branch master https://github.com/containernet/containernet.git

#  We opened a pull request (#243) to make this edit available directly from upstream. See the PR for further details.
cp hack/node.py containernet/mininet/node.py
cp hack/install.yml containernet/ansible/install.yml

sudo ansible-playbook -i "localhost," -c local containernet/ansible/install.yml -e 'ansible_python_interpreter=/usr/bin/python3'

sudo pip3 install -r requirements.txt