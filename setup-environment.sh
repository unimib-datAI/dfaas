#!/bin/bash

set -e

DOCKER_VERSION=$1
SYSBOX_VERSION=$2
SHIFTFS_BRANCH=$3

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

sudo groupadd docker
sudo usermod -aG docker "$USER"

docker version

DOCKER_CONFIG=${DOCKER_CONFIG:-$HOME/.docker}
mkdir -p "$DOCKER_CONFIG"/cli-plugins
curl -SL https://github.com/docker/compose/releases/download/v2.2.3/docker-compose-linux-x86_64 -o "$DOCKER_CONFIG"/cli-plugins/docker-compose
chmod +x "$DOCKER_CONFIG"/cli-plugins/docker-compose

docker compose version

wget https://downloads.nestybox.com/sysbox/releases/v"$SYSBOX_VERSION"/sysbox-ce_"$SYSBOX_VERSION"-0.linux_amd64.deb
sudo apt-get install jq
sudo apt-get install ./sysbox-ce_"$SYSBOX_VERSION"-0.linux_amd64.deb
sudo systemctl status sysbox -n20
docker info | grep -i runtime

git clone -b "$SHIFTFS_BRANCH" https://github.com/toby63/shiftfs-dkms.git shiftfs-"$SHIFTFS_BRANCH"
cd "$SHIFTFS_BRANCH"-k54
./update1
sudo make -f Makefile.dkms
modinfo shiftfs

