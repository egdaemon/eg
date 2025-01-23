#!/bin/bash

set -e

echo "installing google cloud sdk apt credentials ${1}"

curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --dearmor -o ${1}/etc/apt/trusted.gpg.d/cloud.google.gpg
echo "deb [signed-by=/etc/apt/trusted.gpg.d/cloud.google.gpg] https://packages.cloud.google.com/apt cloud-sdk main" | sudo tee ${1}/etc/apt/sources.list.d/google-cloud-sdk.list
