### EG daemon

the only developer productivity focused compute (ci/cd|agent harnesses|compute workload) platform. It focus' on day to day usability with low user friction while providing a powerful computation and orchestration.

### Features

- [x] Homelab functionality as a feature class usecase.
- [x] Highly extensionable due to its entirely virtualized compute platform.
- [x] at cost storage and compute for personal accounts.
- [x] only compute platform that truly focuses on the development lifecycle providing functionality for development, ML training, testing, package builds, and deployments
- [x] local first design, your compute workloads should be able to run as easily locally as they do in the orchestration layer.
- [x] high focus on security. we identify [problems](https://www.egdaemon.com/posts/2024.09.04.secret.scrubbing.misfeature/index.html) before [they happen](https://www.bleepingcomputer.com/news/security/supply-chain-attack-on-popular-github-action-exposes-ci-cd-secrets/).

[Read](https://www.egdaemon.com/posts/2025.01.30.introducing.egd/index.html) the release announcement for a detailed overview of functionality and roadmap.

[Documentation](https://www.egdaemon.com/docs/index.html) for installation/setup guides and package api documentation.

### Installation

The latest [release](https://github.com/egdaemon/eg/releases/latest) provides packages for ubuntu and archlinux (see the [installation guide](https://www.egdaemon.com/docs/installation) for more details).

#### ubuntu

with [gh](https://cli.github.com):

```bash
d=$(mktemp -d) && gh release download --repo egdaemon/eg --pattern "*_$(dpkg --print-architecture).deb" --dir "$d" && sudo apt-get install "$d"/*.deb
```

with curl:

```bash
d=$(mktemp -d) && curl -fsSL https://api.github.com/repos/egdaemon/eg/releases/latest | jq -r --arg a "$(dpkg --print-architecture)" '.assets[] | select(.name | endswith("_" + $a + ".deb")) | .browser_download_url' | xargs curl -fsSL --output-dir "$d" -O && sudo apt-get install "$d"/*.deb
```

#### archlinux

from [aur](https://cli.github.com):

```bash
yay -S egd
```

from [gh](https://cli.github.com):

```bash
d=$(mktemp -d) && gh release download --repo egdaemon/eg --pattern "egd-v*-$(uname -m).pkg.tar.zst" --dir "$d" && sudo pacman -U "$d"/*.pkg.tar.zst
```

from curl:

```bash
d=$(mktemp -d) && curl -fsSL https://api.github.com/repos/egdaemon/eg/releases/latest | jq -r --arg a "$(uname -m)" '.assets[] | select((.name | startswith("egd-v")) and (.name | endswith("-" + $a + ".pkg.tar.zst"))) | .browser_download_url' | xargs curl -fsSL --output-dir "$d" -O && sudo pacman -U "$d"/*.pkg.tar.zst
```
