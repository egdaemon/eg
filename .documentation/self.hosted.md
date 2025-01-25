---
created: 2024-02-28T05:00:00.000Z
updated: 2024-02-28T05:00:00.000Z
title: 'Self Hosted Runners'
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

## authorize the signing key
eg register

# register the signing seed to authorize nodes. this seed is a sensitive value.
eg actl authorize seed 00000000-0000-0000-0000-000000000001

## command line
eg daemon --account="00000000-0000-0000-0000-000000000000" --seed="00000000-0000-0000-0000-000000000001"

## on a workload server

currently we only support ubuntu. contact us if you want support on another platform, there are not any blockers for other distributions, we just havent built/published packages for them yet.

```bash
# install the software.
apt-get install eg egbootstrap

# bootstrap the resources the runner can use.
eg actl bootstrap env runner | sudo tee /etc/eg/runner.env
# settings for the daemon that specify the account and the credentials registered above.
eg actl bootstrap env daemon --account="00000000-0000-0000-0000-000000000000" --seed="00000000-0000-0000-0000-000000000001" | sudo tee /etc/eg/daemon.env

# enable the service and start it immediately
systemctl enable --now eg-runner.service
```

assuming everything was done correctly you'll see an eg-runner instance show up https://console.egdaemon.com/c
