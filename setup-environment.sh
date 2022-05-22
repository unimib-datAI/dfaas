#!/bin/bash

set -e

DOCKER_VERSION=$1
SYSBOX_VERSION=$2
SHIFTFS_BRANCH=$3

sudo apt-get update
sudo apt-get install -yy \
    jq \
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

systemctl enable docker
systemctl start docker

sudo apt-get install docker-compose-plugin

docker compose version

curl -SL https://downloads.nestybox.com/sysbox/releases/v"$SYSBOX_VERSION"/sysbox-ce_"$SYSBOX_VERSION"-0.linux_amd64.deb -o sysbox-ce.deb
sudo apt-get install -yy ./sysbox-ce.deb
docker info | grep -i runtime

git clone -b "$SHIFTFS_BRANCH" https://github.com/toby63/shiftfs-dkms.git shiftfs-"$SHIFTFS_BRANCH"
cd shiftfs-"$SHIFTFS_BRANCH"
./update1
sudo make -f Makefile.dkms
modinfo shiftfs