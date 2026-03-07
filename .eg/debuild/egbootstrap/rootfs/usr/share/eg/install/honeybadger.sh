#!/bin/bash
set -e

echo installing honeybadger

export GOPATH=/tmp/gopath
export GOMODCACHE=/tmp/gomodcache
export GOCACHE=/tmp/gocache
export GOBIN=/usr/local/bin

go install github.com/honeybadger-io/cli/cmd/hb@latest
