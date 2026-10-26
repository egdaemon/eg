---
created: 2024-02-28T05:00:00.000Z
updated: 2024-02-28T05:00:00.000Z
title: "Self Hosted Runners"
index: 2
---

## Self Hosted Runners

Since eg strives to make operations as cost effective and easy to use as possible
self-hosted runners should be considered an option of last resort to satisfy business
needs. that being said this installation guide covers most of the necessary steps and
packages you'll need to install. Most of the work necessary is contained within the
published eg packages publishes.

once you've installed the eg package you'll need to specify some environment settings
like your account, the maximum available resources you want to grant the daemon.

## command line

eg daemon --account="00000000-0000-0000-0000-000000000000" --seed="00000000-0000-0000-0000-000000000001"

## on a workload server

currently we only support ubuntu. contact us if you want support on another platform, there are not any blockers for other distributions, we just havent built/published packages for them yet.

```bash
# install the software.
apt-get install egworkload

# register with eg.
eg register

# the account id and signing seed are sensitive values, so rather than passing them as
# plaintext --account/--seed flags, store them as a secret and load them into the environment
# before eg actl bootstrap/eg actl authorize resolve their flags, via `eg secrets env`. this
# example uses the file:// scheme for simplicity; production usage should use a proper secret
# manager instead, e.g. gcpsm:// (Google Secret Manager) or awssm:// (AWS Secrets Manager) —
# see `eg secrets --help` for the full list of supported schemes.
printf 'EG_ACCOUNT=00000000-0000-0000-0000-000000000000\nEG_ENTROPY_SEED=00000000-0000-0000-0000-000000000001\n' \
  | eg secrets update file:///tmp/daemon.secret

# bootstrap the resources the runner can use.
eg actl bootstrap env runner | sudo tee /etc/eg/runner.env

# settings for the daemon that specify the account and the credentials registered above.
# secrets are loaded into the environment before eg actl bootstrap resolves its flags, so
# --account/--seed don't need to be (and can't accidentally leak as) plaintext flags here.
eg secrets env --uri file:///tmp/daemon.secret eg actl bootstrap env daemon | sudo tee /etc/eg/daemon.env

# clean up the plaintext secret file now that it's no longer needed.
rm -f /tmp/daemon.secret

# build the runner's container.
sudo systemctl start eg-runner-build.service
# enable the service and start it immediately
sudo systemctl enable --now eg-runner.service
# optional gpu service you can run instead.
# sudo systemctl enable --now eg-runner-gpu.service
```

```bash
# authorize the node's signing seed
# this needs to happen on the device you have a
# profile with proper permissions in order to allow
# server to access your eg account.
eg actl authorize seed "$EG_ENTROPY_SEED"
```

assuming everything was done correctly you'll see an eg-runner instance show up https://console.egdaemon.com/c

## overview of /etc/eg/daemon.env

`eg actl bootstrap env daemon` generates this file, every setting is optional except the account. the file is read
by systemd (`EnvironmentFile=`) and by the package's postinst, so keep one `KEY=value` per line. comments
go on their own line, systemd does not strip a trailing `# comment` from a value.

### user

settings you are expected to manage, each has a matching `eg actl bootstrap env daemon` flag.

```bash
# (--account) account to register the runner with.
EG_ACCOUNT=00000000-0000-0000-0000-000000000000

# (--seed) seed for the node's consistent signing identity. sensitive, load it via `eg secrets env`.
# authorize it from your own machine with: eg actl authorize seed "${EG_ENTROPY_SEED}"
EG_ENTROPY_SEED=00000000-0000-0000-0000-000000000001

# (--cores, --memory, --disk, --vram) resources made available to workloads. cores is a vCPU count, memory/disk/vram are in bytes.
# bootstrap raises cores to the detected cpu count and vram to the detected gpu memory,
# and writes memory and disk from its flags.
EG_RESOURCES_CORES=8
EG_RESOURCES_MEMORY=17179869184
EG_RESOURCES_DISK="100GiB"
EG_RESOURCES_VRAM=0

# (--label) comma separated labels (up to 10) assigned to this runner, used to target it from workloads.
EG_LABELS="gpu,fast-disk"

# (--workers) maximum number of workloads run concurrently. default 80% of the cpus, minimum 1.
EG_COMPUTE_WORKLOAD_CAPACITY=2

# (--compilers) maximum number of workloads compiled concurrently. default 1.
EG_COMPUTE_COMPILE_CAPACITY=1

# (--target-load) cpu load the scheduler targets before it stops pulling new workloads. default 0.8.
EG_COMPUTE_WORKLOAD_TARGET_LOAD=0.8

# (--api / --no-api) accept workloads pushed directly to this runner (POST /c/upload, POST /c/enqueue)
# and advertise it to the control plane. default true.
EG_COMPUTE_API_ENABLED=true

# (--mdns-advertise) advertise this daemon on the local network (multicast dns) so clients on the same network
# can upload to it directly. requires EG_COMPUTE_API_ENABLED. default false.
EG_MDNS_ADVERTISE=true

# (--lan-peer-ttl) how long peers on the local network are remembered after they were last seen. default 720h.
EG_LAN_PEER_TTL="720h"

# (--restart-on-upgrade) restart a running daemon when the eg package is upgraded: auto (default), yes or no.
# auto currently behaves like yes. a stopped daemon is never started.
EG_RESTART_ON_UPGRADE=auto
```

### dev

settings for developing and debugging eg itself. there is no bootstrap flag for these, edit the file by hand.
you should not need them to run a runner.

```bash
# upper bound for the delay between checks for new workloads. default 1m.
EG_COMPUTE_SCHEDULER_MAXIMUM_DELAY=1m

# minimum delay between pings to the control plane. default 15m.
EG_COMPUTE_PING_MINIMUM_DELAY=15m

# increase log verbosity, 0 is the default.
EG_LOGGING_VERBOSITY=0
```
