#!/bin/bash
set -e

git clone https://github.com/nsqio/nsq /tmp/nsq
cd /tmp/nsq
make install