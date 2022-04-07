#!/bin/sh
# Generate dfaasagent.env file with all the env vars prefixed with `AGENT_`.
# This file is used by the systemd unit dfaasagent.service.
env | grep ^AGENT_ > /agent/dfaasagent.env
exec /sbin/init --log-level=err