---
created: 2024-02-28T05:00:00.000Z
updated: 2024-02-28T05:00:00.000Z
title: 'Installation'
index: 0
---

## Installation

This document assumes you understand your local package manager/host environment and have duckdb, git, podman, gpgme, and golang toolchain 1.23 (or later) installed.

### ubuntu
```bash
add-apt-repository ppa:egdaemon/eg
apt-get install eg
```

### macosx (alpha)
NOTE: currently macosx support for local workloads is in alpha, remote workloads is ready, local compute still needs some adjustments.

```bash
# unable to connect to podman: unix:///run/eg-daemon/podman/podman.sock
# install podman via https://podman.io/, brew unfortunately failed during initial testing.
brew install duckdb git gpgme
podman machine init --now
CGO_ENABLED=1 CGO_LDFLAGS="-L/opt/homebrew/lib" go install -tags no_duckdb_arrow,duckdb_use_lib github.com/egdaemon/eg/cmd/...@latest
```

### linux from source
```bash
go install -tags no_duckdb_arrow github.com/egdaemon/eg/cmd/...@latest
```
