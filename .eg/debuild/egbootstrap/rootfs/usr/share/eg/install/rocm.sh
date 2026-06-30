#!/bin/bash
set -e

echo "installing google cloud gcsfuse apt credentials ${1}"

curl -fsSL https://repo.radeon.com/rocm/rocm.gpg.key | gpg --dearmor | tee /etc/apt/trusted.gpg.d/rocm.gpg > /dev/null
# we use noble and not $(lsb_release -cs) because amd is slow.
echo "deb [arch=amd64 signed-by=/etc/apt/trusted.gpg.d/rocm.gpg] https://repo.radeon.com/rocm/apt/7.2.4 noble main" | tee /etc/apt/sources.list.d/rocm.list
