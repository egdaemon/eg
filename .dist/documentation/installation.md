---
created: 2024-02-28T05:00:00.000Z
updated: 2024-02-28T05:00:00.000Z
title: "Installation"
index: 0
---

## Installation

This document assumes you understand your local package manager/host environment and have duckdb, git, podman, gpgme, btrfs-progs and golang toolchain 1.23 (or later) installed.

### ubuntu

Install the latest [release](https://github.com/egdaemon/eg/releases/latest) package.

with [gh](https://cli.github.com):

```bash
d=$(mktemp -d) && gh release download --repo egdaemon/eg --pattern "*_$(dpkg --print-architecture).deb" --dir "$d" && sudo apt-get install "$d"/*.deb
```

with curl:

```bash
d=$(mktemp -d) && curl -fsSL https://api.github.com/repos/egdaemon/eg/releases/latest | jq -r --arg a "$(dpkg --print-architecture)" '.assets[] | select(.name | endswith("_" + $a + ".deb")) | .browser_download_url' | xargs curl -fsSL --output-dir "$d" -O && sudo apt-get install "$d"/*.deb
```

Ubuntu users can alternatively use the PPA:

```bash
add-apt-repository ppa:egdaemon/eg
apt-get install eg
```

### archlinux

Install the latest [release](https://github.com/egdaemon/eg/releases/latest) package.

with [gh](https://cli.github.com):

```bash
d=$(mktemp -d) && gh release download --repo egdaemon/eg --pattern "egd-v*-$(uname -m).pkg.tar.zst" --dir "$d" && sudo pacman -U "$d"/*.pkg.tar.zst
```

with curl:

```bash
d=$(mktemp -d) && curl -fsSL https://api.github.com/repos/egdaemon/eg/releases/latest | jq -r --arg a "$(uname -m)" '.assets[] | select((.name | startswith("egd-v")) and (.name | endswith("-" + $a + ".pkg.tar.zst"))) | .browser_download_url' | xargs curl -fsSL --output-dir "$d" -O && sudo pacman -U "$d"/*.pkg.tar.zst
```

### macosx (beta)

NOTE: currently macosx support for local workloads is in beta, remote workloads is ready.

```bash
# unable to connect to podman: unix:///run/eg-daemon/podman/podman.sock
# install podman via https://podman.io/, brew unfortunately failed during initial testing.
brew install duckdb git gpgme
podman machine init --now
CGO_ENABLED=1 CGO_LDFLAGS="-L/opt/homebrew/lib" go install -tags duckdb_use_lib github.com/egdaemon/eg/cmd/...@latest
```

### nixos

NOTE: nixos isn't directly supported at this time, and is community driven.

shell.nix:

```bash
let
  nixpkgs = fetchTarball "https://github.com/NixOS/nixpkgs/tarball/nixos-24.11";
  pkgs = import nixpkgs {};
in
  pkgs.mkShell {
    packages = with pkgs; [go duckdb gpgme btrfs-progs];

    PKG_CONFIG_PATH = pkgs.lib.concatStringsSep ":" ["${pkgs.gpgme.dev}/lib/pkgconfig"];
  }
```

### linux from source

```bash
go install github.com/egdaemon/eg/cmd/...@latest
```
