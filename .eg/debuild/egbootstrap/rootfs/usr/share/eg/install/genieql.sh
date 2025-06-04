#!/bin/bash
set -e

echo installing genieql

export GOPATH=/tmp/gopath
export GOMODCACHE=/tmp/gomodcache
export GOCACHE=/tmp/gocache
export GOBIN=/usr/local/bin

git clone --single-branch -b main https://github.com/james-lawrence/genieql.git && go install -C genieql ./cmd/...
rm -rf genieql
