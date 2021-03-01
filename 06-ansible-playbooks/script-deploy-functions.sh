#!/bin/sh

set -e

cd $(dirname "$0")

# Running in Cygwin. We assume that we are on Windows and WSL is installed
# and Ansible is installed inside WSL

# winpty wsl -e ansible-playbook -Kk -i hosts.yml playbook-deploy-functions.yml # Niente piu' -Kk, ora la password e' salvata nell'inventory hosts.yml
winpty wsl -e ansible-playbook -i hosts.yml playbook-deploy-functions.yml
