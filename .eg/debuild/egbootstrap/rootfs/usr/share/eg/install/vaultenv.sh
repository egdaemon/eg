#!/bin/bash
set -e

echo installing vaultenv
export GOPATH=/tmp/gopath
export GOMODCACHE=/tmp/gomodcache
export GOCACHE=/tmp/gocache
export GOBIN=/usr/local/bin

go install github.com/james-lawrence/vaultenv/cmd/...@latest
