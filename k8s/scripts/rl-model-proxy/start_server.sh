#!/usr/bin/env bash
#
# This small script creates two tmux sessions: one for the proxy server and one
# for the model server. It assumes the servers are already configured (for
# example, the container running the model server has already been created).
#
# This is an experimental deployment that uses tmux purely for convenience and
# testing purposes.

set -euo pipefail

# Use := parameter expansion: it assigns the default value to the variable if
# the environment variable is unset or empty. Also with the ":" no op command,
# we can have a one-line expression.
: "${PROXY_SESSION:=rl-model-proxy}"
: "${SERVE_SESSION:=rl-model-serve}"

: "${PROXY_DIR:=$HOME/dfaas/k8s/scripts/rl-model-proxy}"
: "${CONTAINER_NAME:=rl-agent-v2}"

# Create the PROXY_SESSION.
if tmux has-session -t "$PROXY_SESSION" 2>/dev/null; then
    echo "tmux session '$PROXY_SESSION' already exists"
else
    tmux new-session -d -s "$PROXY_SESSION"

    # Start the proxy server.
    tmux send-keys -t "$PROXY_SESSION" "cd $PROXY_DIR" C-m
    tmux send-keys -t "$PROXY_SESSION" "./rl-model-proxy -listen :8080 -target http://localhost:7999" C-m

    echo "Started $PROXY_SESSION"
fi

# Create the SERVE_SESSION.
if tmux has-session -t "$SERVE_SESSION" 2>/dev/null; then
    echo "tmux session '$SERVE_SESSION' already exists"
else
    tmux new-session -d -s "$SERVE_SESSION"

    # Start the model server.
    tmux send-keys -t "$SERVE_SESSION" "podman start $CONTAINER_NAME && podman logs -f $CONTAINER_NAME" C-m

    echo "Started $SERVE_SESSION"
fi
