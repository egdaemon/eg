### EG daemon

the only developer productivity focused compute (ci/cd) platform. It focus' on day to day usability with low user friction while providing a powerful computation and orchestration.

### Features
- [x] at cost storage and compute for solo developers.
- [ ] (WIP) only compute platform that truly focuses on the development lifecycle providing functionality for development, ML training, testing, package builds, and deployments
- [x] local first design, your compute workloads should be able to run as easily locally as they do in the orchestration layer.
- [x] high focus on security. we identify [problems](https://www.egdaemon.com/posts/2024.09.04.secret.scrubbing.misfeature/index.html) before [they happen](https://www.bleepingcomputer.com/news/security/supply-chain-attack-on-popular-github-action-exposes-ci-cd-secrets/).

[Read](https://www.egdaemon.com/posts/2025.01.30.introducing.egd/index.html) the release announcement for a detailed overview of functionality and roadmap.

[Documentation](https://www.egdaemon.com/docs/index.html) for installation/setup guides and package api documentation.

### useful debugging commands
```bash
# setup container that matches deployed environment for building/testing.
podman run -it --volume ~/go/bin:/root/go/bin:rw --volume .:/workload eg:latest /bin/bash
```