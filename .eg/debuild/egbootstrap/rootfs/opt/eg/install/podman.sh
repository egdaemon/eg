#!/bin/bash
set -e

echo "installing podman apt credentials"
echo 'deb https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/unstable/xUbuntu_24.04/ /' | tee /etc/apt/sources.list.d/podman.list
curl -fsSL https://download.opensuse.org/repositories/devel:/kubic:/libcontainers:/unstable/xUbuntu_24.04/Release.key | gpg --dearmor -o /etc/apt/trusted.gpg.d/podman.gpg
