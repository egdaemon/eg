#!/bin/bash
set -e

echo "installing yarn apt credentials ${1}/etc/apt/"

mkdir -p ${1}/etc/apt/{trusted.gpg.d,sources.list.d}
curl -sSL https://dl.yarnpkg.com/debian/pubkey.gpg | gpg --dearmor -o ${1}/etc/apt/trusted.gpg.d/yarn.gpg
echo "deb [signed-by=/etc/apt/trusted.gpg.d/yarn.gpg] https://dl.yarnpkg.com/debian/ stable main" | tee ${1}/etc/apt/sources.list.d/yarn.list
