#!/bin/bash
set -e

echo "installing podman apt credentials ${1}/etc/apt/"
echo 'deb https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/unstable/xUbuntu_24.04/ /' | tee ${1}/etc/apt/sources.list.d/podman.list
curl -fsSL https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/unstable/xUbuntu_24.04/Release.key | gpg --dearmor -o ${1}/etc/apt/trusted.gpg.d/podman.gpg
