---
created: 2026-11-01T05:00:00.000Z
updated: 2026-11-01T05:00:00.000Z
title: "Secrets"
index: 2
---

## Secrets

eg ships with a simple secret manager, `eg secrets`, for the values you don't want to
pass as plaintext flags or commit next to your code: accounts, seeds, tokens. a secret is
referenced by a uri, the scheme of the uri picks where it is stored.

| scheme | example | notes |
| --- | --- | --- |
| `chachasm` | `chachasm://passphrase@.secrets/daemon.env` | a file encrypted with the passphrase. the passphrase is part of the uri, so it is visible in `ps` while a command runs, keep it in an environment variable. the directory must already exist. |
| `file` | `file:./daemon.secret`, `file:///etc/eg/daemon.secret` | plaintext, written with mode 0600. |
| `gcpsm` | `gcpsm://project/name`, `gcpsm://project/name/3` | Google Secret Manager, the version defaults to `latest`. |
| `awssm` | `awssm://name?region=us-east-1` | AWS Secrets Manager, a version longer than 16 characters is a version id otherwise it is a stage such as `AWSCURRENT`. |

the commands are `create`, `read`, `edit`, `env` and `b64`, see `eg secrets --help`.
secrets used with `env` are `KEY=VALUE` lines.

## examples

```bash
# create a plaintext secret from stdin.
printf 'EG_ACCOUNT=00000000-0000-0000-0000-000000000000\nEG_ENTROPY_SEED=00000000-0000-0000-0000-000000000001\n' | eg secrets create file:./daemon.secret

# create from a file. an existing secret is never changed unless --overwrite
# is given.
eg secrets create --input ./daemon.env file:///etc/eg/daemon.secret
eg secrets create --overwrite --input ./daemon.env file:///etc/eg/daemon.secret

# create an encrypted secret, the passphrase comes from the environment. it is
# part of the uri, so it is visible in `ps` while the command runs.
export DEMOPASSPHRASE="..."
mkdir -p .secrets
eg secrets create "chachasm://${DEMOPASSPHRASE}@.secrets/daemon.env" < daemon.env

# edit interactively with $EDITOR, the secret is rewritten only when the
# contents changed. a secret that doesn't exist yet is created.
EDITOR=vim eg secrets edit "chachasm://${DEMOPASSPHRASE}@.secrets/daemon.env"

# read a secret to stdout, or to a file.
eg secrets read "chachasm://${DEMOPASSPHRASE}@.secrets/daemon.env"
eg secrets read --output ./daemon.env file:./daemon.secret

# a missing secret is an error, unless --allow-missing is given: it is skipped
# with a warning and the others are still read.
eg secrets read --allow-missing file:./base.secret file:./optional.secret

# copy a secret between schemes, or merge several into a new one. multiple uris
# are concatenated, one after another.
eg secrets read file:./base.secret file:./prod.secret | eg secrets create "chachasm://${DEMOPASSPHRASE}@.secrets/merged.env"
eg secrets read gcpsm://my-project/eg-daemon "awssm://eg-daemon?region=us-east-1" | eg secrets create gcpsm://my-project/eg-merged

# pipe into edit to append to an existing secret, then review it in the
# editor.
eg secrets read file:./base.secret file:./prod.secret | eg secrets edit "chachasm://${DEMOPASSPHRASE}@.secrets/merged.env"

# replace the destination if it already exists.
eg secrets read file:./base.secret file:./prod.secret | eg secrets create --overwrite "chachasm://${DEMOPASSPHRASE}@.secrets/merged.env"

# run a command with the secret loaded into its environment.
eg secrets env --uri file:./daemon.secret eg actl bootstrap env daemon

# several secrets are merged. -e adds or overrides a value, a bare key is
# copied from the current environment.
eg secrets env --uri file:./base.secret --uri "chachasm://${DEMOPASSPHRASE}@.secrets/prod.env" -e EG_LABELS=prod -e HOME env

# no command prints the resolved KEY=VALUE lines. this prints secret values.
eg secrets env --uri file:./daemon.secret

# hosted secret managers: a pinned gcp version, an aws secret in a region, and
# creating one in each.
eg secrets read gcpsm://my-project/eg-daemon/3
eg secrets read "awssm://eg-daemon?region=us-east-1"
eg secrets create gcpsm://my-project/eg-daemon < daemon.env
eg secrets create "awssm://eg-daemon?region=us-east-1" < daemon.env

# inside a systemd unit. systemd can't read an encrypted file as an
# EnvironmentFile, so decrypt inside the unit and pass PATH through.
systemd-run --user --pty --wait --working-directory=$PWD --setenv=PATH -p RuntimeDirectory=eg-demo -p CacheDirectory=eg-demo eg secrets env --uri "chachasm://${DEMOPASSPHRASE}@.dist/demo/demo.daemon.env" eg daemon
```

## notes

- `edit` reads stdin when it is not a terminal and appends it to the secret before
  the editor opens. when stdin is a terminal you can simply type, nothing is read.
- `eg secrets b64` base64 url encodes stdin, e.g.) `printf %s value | eg secrets b64 -o out`.
- scheduled workloads can only reference `gcpsm` and `awssm` secrets, and without a
  passphrase in the uri. local files and passphrases don't belong in workloads.
