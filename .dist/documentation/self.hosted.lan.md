---
created: 2026-10-20T05:00:00.000Z
updated: 2026-10-20T05:00:00.000Z
title: "Local Network Runners"
index: 3
---

## Local Network Runners (alpha)

Upload workloads straight to a self hosted runner on the same network as your machine, found with multicast DNS
(mDNS), instead of going through a relay. Both sides are opt in: the runner chooses to be found and your machine
chooses to use it.

## quick start

on the runner, enable advertising and restart the daemon. the secret is the one from the Self Hosted Runners guide.

```bash
eg secrets env --uri file:///tmp/daemon.secret eg actl bootstrap env daemon --mdns-advertise | sudo tee /etc/eg/daemon.env
sudo systemctl restart eg-daemon.service
```

on your machine, find the runner and upload to it.

```bash
# find runners on the network and remember them.
eg compute lan scan

# try local runners first, then the control plane's suggestions, then the direct upload to control plane.
eg compute upload --lan=2 my-workload
```

the runner needs multicast (UDP 5353) to reach your network, see the network requirements below.

## set up the runner

advertising is a daemon setting. any of these turn it on, `eg actl bootstrap env daemon` is the one that writes the
daemon's environment file for you.

```bash
# write the environment file with advertising on.
eg secrets env --uri file:///tmp/daemon.secret eg actl bootstrap env daemon --mdns-advertise | sudo tee /etc/eg/daemon.env

# or add it to an existing environment file.
echo 'EG_MDNS_ADVERTISE=true' | sudo tee -a /etc/eg/daemon.env

# or pass the flag when running the daemon yourself.
eg daemon --mdns-advertise
```

restart whichever service you run, `eg-daemon.service` or the containerized `eg-runner.service`.

a runner only advertises when it accepts direct uploads (`--uploadable`, the default).

### network requirements

advertising needs multicast traffic (UDP port 5353) to reach the network from the place the daemon runs.

- `eg-daemon.service` runs on the host and already allows it. the unit denies multicast and link-local traffic and makes
  an exception for the two mDNS groups only, `IPAddressAllow=224.0.0.251 ff02::fb`. because link-local traffic is still
  denied, the runner needs an IPv4 address, or an IPv6 address that isn't link-local (`fe80::/10`), on that network.
- the containerized runner (`eg-runner.service`) runs on a podman bridge network (`egnet`), which neither forwards
  multicast nor gives other machines a route to the container. making it reachable (for example host networking) is up to
  whoever administers the machine, eg doesn't change it for you.
- a firewall or network between the machines can still drop multicast, see troubleshooting below.

## upload using the local network

```bash
# local runners first (priority 2), then the control plane's suggestions (priority 1), then the cluster.
eg compute upload --lan=2 my-workload

# the other way around.
eg compute upload --lan=1 --p2p=2 my-workload

# straight to the cluster.
eg compute upload --lan=0 --p2p=0 my-workload

# turn local runners on for every upload from this machine.
export EG_LAN_PRIORITY=2
```

higher numbers are tried first and `0` disables a strategy. uploading to the cluster is always the last resort.

| strategy                                                 | flag    | environment       | default     |
| -------------------------------------------------------- | ------- | ----------------- | ----------- |
| runners remembered from the local network                | `--lan` | `EG_LAN_PRIORITY` | 0 (off)     |
| runners the control plane suggests, over the p2p network | `--p2p` | `EG_P2P_PRIORITY` | 1           |
| upload to the cluster                                    | -       | -                 | always last |

if no local runner is remembered, none accepts the workload, or the network is unavailable, the upload carries on with
the next strategy. the control plane is always needed, local discovery doesn't work without it.

## manage remembered systems

```bash
# list the systems this machine remembers, the peer id is the first column.
eg compute lan ls

# scan the network and remember what is found, without uploading anything.
eg compute lan scan

# the same, then list.
eg compute lan ls --scan

# stop using a system without forgetting it, and use it again later.
eg compute lan upd <peer-id> --disable
eg compute lan upd <peer-id> --enable

# describe a system.
eg compute lan upd <peer-id> --description "the build box"

# remember a system for a specific time, forever, or the default again.
eg compute lan upd <peer-id> --ttl 720h
eg compute lan upd <peer-id> --ttl infinity
eg compute lan upd <peer-id> --reset-ttl

# forget a system. it comes back the next time it's seen.
eg compute lan rm <peer-id>
```

a runner keeps its own list of the machines that have reached it. manage it on the runner with `--runner`, as the
user the daemon runs as and pointing at the daemon's cache directory, for the packaged service:

```bash
sudo -u egd eg compute lan ls --runner --cache-directory /var/cache/eg-daemon
sudo -u egd eg compute lan upd --runner --cache-directory /var/cache/eg-daemon <peer-id> --disable
```

## settings

| setting                             | flag                                                                          | environment                           | default        |
| ----------------------------------- | ----------------------------------------------------------------------------- | ------------------------------------- | -------------- |
| advertise the daemon on the network | `eg daemon --mdns-advertise`                                                  | `EG_MDNS_ADVERTISE`                   | off            |
| local network priority              | `eg compute upload --lan`                                                     | `EG_LAN_PRIORITY`                     | 0 (off)        |
| p2p network priority                | `eg compute upload --p2p`                                                     | `EG_P2P_PRIORITY`                     | 1              |
| how long a scan runs                | `--lan-scan-timeout` (upload), `--scan-timeout` (`lan scan`, `lan ls --scan`) | -                                     | 3s             |
| how long systems are remembered     | `--lan-peer-ttl`                                                              | `EG_LAN_PEER_TTL`                     | 720h (30 days) |
| where the list is stored            | `--cache-directory` (upload, `eg compute lan`), `--directory` (daemon)        | `EG_CACHE_DIRECTORY` (not the daemon) | see below      |

`--lan-peer-ttl` is accepted by `eg daemon`, `eg compute upload` and the `eg compute lan` commands, and
`eg actl bootstrap env daemon --lan-peer-ttl 168h` writes it to the daemon's environment file. a duration set on a
single system with `eg compute lan upd --ttl` takes priority over it.

## troubleshooting

- **nothing is found.** mDNS uses multicast on UDP port 5353. check that a firewall isn't dropping it and that both
  machines are on the same network segment. many guest wifi networks, vlans and vpns block multicast. see
  the network requirements above for the runner side.
- **a system is in the list but isn't used.** check `eg compute lan ls` for `disabled`, that the control plane considers
  the runner able to run the workload (cores, memory, architecture and labels), and that it was seen recently enough
  to have an address (`eg compute lan scan`). systems the control plane told you about but that were never seen on
  your network are listed without an address and are only reachable the regular way.
- **a runner stopped showing up.** it is dropped from the list after not being seen for the retention period,
  run `eg compute lan scan` to find it again.
- **uploads are slower than before.** run with `--lan=0` (the default) to rule out the local network. a remembered runner
  whose address is no longer reachable has to time out before the next strategy is tried, `eg compute lan rm <peer-id>`
  forgets it.
- **a command reports the database is locked.** only one process can use a list at a time. an upload (or the daemon,
  for a few milliseconds per connection) is using it, run the command again.

## how it works

- **authorization is unchanged.** you still need an eg account, your account still decides which runners you may use,
  and the control plane still issues and verifies the token that allows a workload to be uploaded. local discovery only
  changes how your machine finds and reaches the runner.
- **the runner you reach is the one you expect.** a runner with advertising on announces its peer id and its local
  addresses. the peer id is the runner's public key and the connection is authenticated against it, so it is the runner
  the control plane told you about, and only runners the control plane suggests are used.
- **uploads try the strategies in priority order.** `eg compute upload` asks the control plane which runners can run
  your workload as usual, then tries the strategies in the order of their priority. the first runner to accept wins.
- **scanning never delays an upload.** when `--lan` is above 0 a scan runs in the background while your workload is
  being built and remembers whatever it finds. the upload itself only uses what is already remembered, so a runner your
  machine has never seen is normally used by that upload if the scan found it while the workload was building, otherwise
  by the next one.
- **every system seen is remembered.** along with its resources and addresses, in a small cache on each machine. the
  control plane's candidates are remembered too, which is how resources are known without asking the runner.
- **the runner's list is bookkeeping.** a runner records the machines that have reached it and refuses the ones you
  disable. it is on top of the control plane, not a replacement for it: a machine without a valid token can't upload
  whether or not it is listed.

### how long systems are remembered and where

systems are forgotten when they haven't been seen for 30 days, and being seen again refreshes that.

the list is stored in the cache directory: `$CACHE_DIRECTORY` when set (systemd sets this for services), otherwise
`$XDG_CACHE_HOME`, otherwise `~/.cache/eg`. the client uses `lan.client.duckdb` and the daemon `lan.runner.duckdb`, the
daemon in its `--directory` cache directory, which for the packaged `eg-daemon.service` is `/var/cache/eg-daemon`
(`CacheDirectory=` in the unit). it is a cache, deleting it only means systems have to be discovered again, but it also
clears any systems you disabled.
