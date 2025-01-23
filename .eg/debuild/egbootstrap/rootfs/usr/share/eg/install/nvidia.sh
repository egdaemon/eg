#!/bin/bash
set -e
# we have manually added these keys into the base image's filesystem
# this script is just for documenting the commands used for posterity and prosperity
echo installing nvidia apt credentials

# nvidia docker apt repositories
curl -s -L https://nvidia.github.io/nvidia-docker/ubuntu22.04/nvidia-docker.list | sudo tee /etc/apt/sources.list.d/nvidia-docker.list
curl -fsSL https://nvidia.github.io/nvidia-docker/gpgkey | gpg --dearmor -o /etc/apt/keyrings/nvidia-docker.gpg


sudo apt-get update && sudo apt-get install -y nvidia-container-toolkit
