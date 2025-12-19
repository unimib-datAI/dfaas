#! /bin/bash

# read the last tested configuration
echo Reading last tested configuration...

FILE=$(ls -t ../output/results* 2>/dev/null | head -n 1)

# Read last line
LAST_LINE=$(tail -n 1 "$FILE")

# Split line into array by comma
IFS=',' read -ra FIELDS <<< "$LAST_LINE"

# Calculate how many fields to consider (ignore last 10)
TOTAL_FIELDS=${#FIELDS[@]}
FIELDS_TO_PROCESS=$((TOTAL_FIELDS - 10))

NAMES=()
RATES=()

i=0
while [ $i -lt $FIELDS_TO_PROCESS ]; do
  NAME="${FIELDS[$i]}"
  if [ "$NAME" == "eat-memory" ]; then
    NAME="cavacal/$NAME"
  fi
  RATE="${FIELDS[$((i + 1))]}"
  NAMES+=("$NAME")
  RATES+=("$RATE")
  i=$((i + 9))  # jump by 9 fields per function block
done

echo Overwriting the configuration.txt file with new starting configuration...

# Write to configuration.txt
{
  IFS=','; echo "${NAMES[*]}"
  IFS=','; echo "${RATES[*]}"
} > configuration.txt

# Resting the remote minikube cluster

# SSH connection info
USER="user" # Replace with username used on the VM
HOST="10.12.38.4" # Replace with IP of the remote VM

# Remote command you want to run on the VM
REMOTE_COMMAND="~/dfaas/metrics_predictions/minikube_builder.sh"

# Run the SSH command
ssh "${USER}@${HOST}" "$REMOTE_COMMAND"

if [ $? -eq 0 ]; then
    echo "Success: Remote script executed successfully."
else
    echo "Error: Remote script execution failed."
    exit 1
fi
