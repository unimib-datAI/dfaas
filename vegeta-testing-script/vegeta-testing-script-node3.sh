#!/bin/bash

set -e

cd $(dirname "$0")

################################################################################

mkdir -p vegeta-results
VEGFOLDER="vegeta-results/$(date +%Y-%m-%d-%H%M%S)"
mkdir -p $VEGFOLDER

################################################################################

docker logs agent1 -ft --tail 50 > $VEGFOLDER/agent1.log 2>&1 &
JOBPID_agent1=$!
echo "Started agent1 log capture with PID $JOBPID_agent1"

docker logs agent2 -ft --tail 50 > $VEGFOLDER/agent2.log 2>&1 &
JOBPID_agent2=$!
echo "Started agent2 log capture with PID $JOBPID_agent2"

docker logs agent3 -ft --tail 50 > $VEGFOLDER/agent3.log 2>&1 &
JOBPID_agent3=$!
echo "Started agent3 log capture with PID $JOBPID_agent3"

docker logs haproxy1 -ft --tail 50 > $VEGFOLDER/haproxy1.log 2>&1 &
JOBPID_haproxy1=$!
echo "Started haproxy1 log capture with PID $JOBPID_haproxy1"

docker logs haproxy2 -ft --tail 50 > $VEGFOLDER/haproxy2.log 2>&1 &
JOBPID_haproxy2=$!
echo "Started haproxy2 log capture with PID $JOBPID_haproxy2"

docker logs haproxy3 -ft --tail 50 > $VEGFOLDER/haproxy3.log 2>&1 &
JOBPID_haproxy3=$!
echo "Started haproxy3 log capture with PID $JOBPID_haproxy3"

VEG_port=8003
VEG_func=funca
VEG_duration=5m
VEG_rate=50 # req/s
VEG_every=200ms
echo "Starting vegeta attack with duration $VEG_duration . . ."
echo "GET http://127.0.0.1:$VEG_port/function/$VEG_func" | \
    vegeta attack -duration=$VEG_duration -rate=$VEG_rate | \
    tee $VEGFOLDER/results.bin | vegeta report -every=$VEG_every
echo "Done!"

cat $VEGFOLDER/results.bin | vegeta encode > $VEGFOLDER/results.json

kill $JOBPID_agent1
kill $JOBPID_agent2
kill $JOBPID_agent3
kill $JOBPID_haproxy1
kill $JOBPID_haproxy2
kill $JOBPID_haproxy3
