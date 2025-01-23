#!/bin/bash
set -e

echo installing systemd alerts
export GOPATH=/tmp/gopath
export GOMODCACHE=/tmp/gomodcache
export GOCACHE=/tmp/gocache
export GOBIN=/usr/local/bin

go install github.com/james-lawrence/systemd-alert/commands/...@latest
