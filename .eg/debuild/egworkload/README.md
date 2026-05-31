# egworkload

Debian meta-package that configures a container for running eg workloads.

Installing this package replaces the multi-step manual container setup (sysusers, tmpfiles,
subuid/subgid ranges, podman socket, sudo) with a single apt install.

## usage

```dockerfile
FROM ubuntu:resolute
ARG DEBIAN_FRONTEND=noninteractive

RUN apt-get update
RUN apt-get install -y software-properties-common build-essential ca-certificates curl
RUN add-apt-repository -n ppa:egdaemon/duckdb ppa:egdaemon/eg

RUN apt-get update
RUN apt-get -y install egworkload
```
