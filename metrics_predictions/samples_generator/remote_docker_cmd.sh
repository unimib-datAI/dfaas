#!/bin/bash

# SSH connection info
USER="thomashg" # Replace with username used on the VM
HOST="10.99.217.210" # Replace with IP of the remote VM

# Check if at least one argument (function name) is provided
if [ "$#" -eq 0 ]; then
  echo "Error: No function names provided."
  exit 1
fi

# Remote command you want to run on the VM
REMOTE_COMMAND="docker exec minikube python3 etc/find-pid.py $@"

# Run the SSH command
ssh "${USER}@${HOST}" "$REMOTE_COMMAND"

# Check if the command was successful
if [ $? -eq 0 ]; then
    echo "Success: $OUTPUT"
else
    echo "Error: Command failed."
    exit 1
fi