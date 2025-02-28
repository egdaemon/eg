#!/bin/bash
set -e

echo "installing github cli repository ${1}"

curl https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo tee ${1}/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
