#!/bin/bash
set -e

echo "installing google cloud gcsfuse apt credentials ${1}"

curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo tee ${1}/usr/share/keyrings/cloud.google.asc
echo "deb https://packages.cloud.google.com/apt gcsfuse-`lsb_release -c -s` main" | sudo tee ${1}/etc/apt/sources.list.d/gcsfuse.list
