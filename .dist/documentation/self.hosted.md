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

# authorize this node's signing seed, reading it from the secret via the wrapper.
# `authorize seed` takes its seed as a positional arg (not a kong default), so it needs a
# shell to expand $EG_ENTROPY_SEED from the environment eg secrets env sets on the child.
eg secrets env --uri file:///tmp/daemon.secret -- \
  sh -c 'eg actl authorize seed "$EG_ENTROPY_SEED"'

# bootstrap the resources the runner can use.
eg actl bootstrap env runner | sudo tee /etc/eg/runner.env

# settings for the daemon that specify the account and the credentials registered above.
# secrets are loaded into the environment before eg actl bootstrap resolves its flags, so
# --account/--seed don't need to be (and can't accidentally leak as) plaintext flags here.
eg secrets env --uri file:///tmp/daemon.secret -- \
  eg actl bootstrap env daemon | sudo tee /etc/eg/daemon.env

# clean up the plaintext secret file now that it's no longer needed.
rm -f /tmp/daemon.secret

# build the runner's container.
systemctl start eg-runner-build.service
# enable the service and start it immediately
systemctl enable --now eg-runner.service
```

assuming everything was done correctly you'll see an eg-runner instance show up https://console.egdaemon.com/c
