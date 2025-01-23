#!/bin/bash

# old known working version.
# curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key --keyring /usr/share/keyrings/cloud.google.gpg add -
# curl -sSO https://dl.google.com/cloudagents/add-google-cloud-ops-agent-repo.sh
# sudo bash add-google-cloud-ops-agent-repo.sh --also-install
curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --dearmor -o /etc/apt/trusted.gpg.d/cloud.google.gpg
curl -sSO https://dl.google.com/cloudagents/add-google-cloud-ops-agent-repo.sh
sudo bash add-google-cloud-ops-agent-repo.shl
