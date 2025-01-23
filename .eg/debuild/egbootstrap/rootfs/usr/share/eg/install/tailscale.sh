#!/bin/bash
set -e

echo "installing tailscale apt credentials to ${1}"

curl -fsSL https://pkgs.tailscale.com/stable/ubuntu/jammy.noarmor.gpg | sudo tee ${1}/usr/share/keyrings/tailscale-archive-keyring.gpg > /dev/null
curl -fsSL https://pkgs.tailscale.com/stable/ubuntu/jammy.tailscale-keyring.list | sudo tee ${1}/etc/apt/sources.list.d/tailscale.list