#!/bin/bash

set -e

cd $(dirname "$0")

if grep -qEi "(Microsoft|WSL)" /proc/version &> /dev/null ; then
    # We are inside WSL

    touch "$HOME/.ssh/known_hosts"

    # Remove entries from known hosts
    ssh-keygen -R "[ctrl.dfaas.lvh.me]:12200"
    echo "Removed successfully [ctrl.dfaas.lvh.me]:12200"
    ssh-keygen -R "[node01.dfaas.lvh.me]:12201"
    echo "Removed successfully [node01.dfaas.lvh.me]:12201"
    ssh-keygen -R "[node02.dfaas.lvh.me]:12202"
    echo "Removed successfully [node02.dfaas.lvh.me]:12202"
    ssh-keygen -R "[node03.dfaas.lvh.me]:12203"
    echo "Removed successfully [node03.dfaas.lvh.me]:12203"

    # Add entries to known hosts
    ssh-keyscan -p 12200 ctrl.dfaas.lvh.me >> "$HOME/.ssh/known_hosts"
    ssh-keyscan -p 12201 node01.dfaas.lvh.me >> "$HOME/.ssh/known_hosts"
    ssh-keyscan -p 12202 node02.dfaas.lvh.me >> "$HOME/.ssh/known_hosts"
    ssh-keyscan -p 12203 node03.dfaas.lvh.me >> "$HOME/.ssh/known_hosts"
else
    # We are inside another shell (probably Git Bash)

    winpty wsl -e "$0"
fi
