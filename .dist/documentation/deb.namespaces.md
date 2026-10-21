---
created: 2026-10-21T05:00:00.000Z
updated: 2026-10-21T05:00:00.000Z
title: "Deb Namespaces"
index: 5
---

## Deb Namespaces

A deb namespace is an apt repository hosted by eg. Packages built by your workloads are published to the namespace and installed with stock apt from `https://{namespace}.{account id}.apt.egdaemon.com`.

### how it works

- you sign the repository. the indexes (`InRelease`, `Release.gpg`) are generated and signed by your gpg key during publish, eg verifies every publish against the public keys registered with the namespace and never holds your private key.
- packages are stored by their sha256 digest, uploading the same package again is free.
- every publish creates a new generation of the namespace atomically. concurrent publishers are detected and retried, existing packages and other distributions are retained.
- namespaces are private by default, apt authenticates with a token. public namespaces can be installed by anyone.

### 1. create the namespace

```bash
# export the public key that will sign the repository.
gpg --armor --export <key id> > signing.asc

# create the namespace, add --public to allow anyone to install from it.
eg deb create tools --keys signing.asc

# list namespaces, show installation instructions, change, or remove them.
eg deb ls
eg deb show tools
eg deb edit tools --no-public --keys rotated.asc
eg deb rm tools
```

### 2. publish from a workload

build binary packages with `egdebuild` and publish them, the signing key must be the key registered with the namespace.

```golang
deb := egdebuild.New(
	"hello",
	"resolute",
	egenv.WorkingDirectory(),
	egdebuild.Option.Version("1.0.:autopatch:"),
	egdebuild.Option.SigningKeyID("<key id>"),
	egdebuild.Option.BuildBinary(10*time.Minute),
)

err := eg.Perform(
	ctx,
	egdebuild.Prepare(egdebuild.Runner(), nil),
	eg.Module(ctx, egdebuild.Runner(), egdebuild.Build(deb), egdebuild.Publish(deb, "tools")),
)
```

publishing requires a token with deb access in the `EG_DEB_TOKEN` environment variable, and access to your gpg agent. both are available to local workloads:

```bash
eg compute local -e EG_DEB_TOKEN="$(eg deb token)" .eg/release
```

the following environment variables adjust the published layout:

| variable          | default | description                                                                   |
| ----------------- | ------- | ----------------------------------------------------------------------------- |
| DEB_COMPONENT     | main    | component of the distribution.                                                |
| DEB_ARCHITECTURES | amd64   | architectures of the indexes, architecture `all` packages are added to each. |

### 3. install

```bash
eg deb show tools
```

prints the commands to configure apt. for a namespace named `tools`:

```bash
sudo tee /etc/apt/keyrings/tools.asc < signing.asc > /dev/null
echo "deb [signed-by=/etc/apt/keyrings/tools.asc] https://tools.<account id>.apt.egdaemon.com resolute main" | sudo tee /etc/apt/sources.list.d/tools.list

# private namespaces only.
printf 'machine tools.<account id>.apt.egdaemon.com\nlogin eg\npassword %s\n' "$(eg deb token)" | sudo tee /etc/apt/auth.conf.d/tools.conf
sudo chmod 600 /etc/apt/auth.conf.d/tools.conf

sudo apt-get update && sudo apt-get install hello
```

### limitations

- tokens issued by `eg deb token` expire after an hour, long lived read tokens for apt clients of private namespaces are not available yet.
- indexes must include an uncompressed or gzip compressed `Packages`, `xz` only indexes are rejected.
- publishing is only supported from local workloads, since signing requires your gpg agent.
