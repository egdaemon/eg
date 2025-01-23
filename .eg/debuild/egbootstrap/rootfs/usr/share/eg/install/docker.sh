#!/bin/bash
set -e

echo "installing docker apt credentials to ${1}/etc/apt/"

mkdir -p ${1}/etc/apt/{trusted.gpg.d,sources.list.d}
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o ${1}/etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee ${1}/etc/apt/sources.list.d/docker.list > /dev/null