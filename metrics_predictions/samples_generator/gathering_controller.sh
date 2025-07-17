#!/bin/bash

while true; do
    echo "Starting shell script..."
    bash restart_data_gathering.sh
    shell_exit_code=$?
    echo "Shell script exited with code $shell_exit_code"

    # Optional: Add a delay between iterations
    sleep 5
    
    echo "Starting Python script..."
    python3 samples-generator.py 200 30s --no-scaphandre
    python_exit_code=$?
    echo "Python script exited with code $python_exit_code"
    
    sleep 5
done