#!/bin/bash

set -e

DOCKER_VERSION=$1
DOCKER_COMPOSE_VERSION=$2
SYSBOX_VERSION=$3
SHIFTFS_BRANCH=$4

sudo apt-get update
sudo apt-get install \
    ca-certificates \
    curl \
    wget \
    gnupg \
    make \
    dkms \
    lsb-release

curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh "$DOCKER_VERSION"

sudo usermod -aG docker "$USER"

DOCKER_CONFIG=${DOCKER_CONFIG:-$HOME/.docker}
mkdir -p "$DOCKER_CONFIG"/cli-plugins
curl -SL https://github.com/docker/compose/releases/download/"$DOCKER_COMPOSE_VERSION"/docker-compose-linux-x86_64 -o "$DOCKER_CONFIG"/cli-plugins/docker-compose
chmod +x "$DOCKER_CONFIG"/cli-plugins/docker-compose

docker compose version

curl -SL https://downloads.nestybox.com/sysbox/releases/v"$SYSBOX_VERSION"/sysbox-ce_"$SYSBOX_VERSION"-0.linux_amd64.deb -o sysbox-ce.deb
sudo apt-get install jq
sudo apt-get install ./sysbox-ce.deb
sudo systemctl status sysbox -n20
docker info | grep -i runtime

git clone -b "$SHIFTFS_BRANCH" https://github.com/toby63/shiftfs-dkms.git shiftfs-"$SHIFTFS_BRANCH"
cd shiftfs-"$SHIFTFS_BRANCH"
./update1
sudo make -f Makefile.dkms
modinfo shiftfs

