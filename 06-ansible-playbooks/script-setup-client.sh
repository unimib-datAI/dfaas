#!/bin/sh

set -e

cd $(dirname "$0")

# Running in Cygwin. We assume that we are on Windows and WSL is installed
# and Ansible is installed inside WSL

winpty wsl -e ansible-galaxy install -r requirements.yml
