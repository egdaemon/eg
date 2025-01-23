#!/bin/bash
set -e

echo installing goose
export GOPATH=/tmp/gopath
export GOMODCACHE=/tmp/gomodcache
export GOCACHE=/tmp/gocache
export GOBIN=/usr/local/bin

go install github.com/pressly/goose/v3/cmd/goose@latest
