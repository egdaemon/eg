# Using eg (guide for agents)

eg is a local-first compute platform for CI/CD, agent harnesses and other workloads. A workload is a Go `main` package that eg compiles to `GOOS=wasip1 GOARCH=wasm` and runs sandboxed. `eg compute local` is meant to behave the same as the hosted service, so verify locally before uploading.

Longer docs live in `.dist/documentation/`: `getting.started.md`, `installation.md`, `custom.containers.md`, `gcp.md`, `workload.analytics.md`, `self.hosted.md`, `self.hosted.lan.md`.

## Rules

- Edit workloads under `.eg/` only. Never edit or search `.eg.workload/` or `.eg.cache/`; they are gitignored build output and hold copies of the repo.
- Verify changes with `eg compute local [module]` instead of assuming they work.
- Secrets: pass them by URI with `--secret <uri>` and nothing else. Never read, print, create, edit or rotate secret contents, never cat secret files, and never dump the environment of a run that carries them. A human manages secrets.
- Do not use `eg compute baremetal` (no isolation) or `--privileged` unless asked.
- Avoid anything marked `Unsafe*` or `Deprecated`, and `eg.DefaultModule()` for real work.

## Where workloads live

`.eg/` at the repo root is its own Go module (`module eg/compute`). Create it with `eg actl bootstrap module`.

- `.eg/main.go` is the default workload.
- Each subfolder is a named workload: `eg compute local tests/envvars` runs `.eg/tests/envvars`.
- Code under `.eg/` can be shared between workloads.
- A container for the workload usually sits beside it as `.eg/Containerfile`.

## Minimal workload

```go
func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	if err := eg.Perform(ctx, HelloWorld); err != nil {
		log.Fatalln(err)
	}
}

func HelloWorld(ctx context.Context, op eg.Op) error {
	log.Println("hello world")
	return nil
}
```

A realistic one: clone the repo, build a container, run go tests inside it.

```go
c1 := eg.Container("eg.ubuntu")
err := eg.Perform(ctx,
	eggit.AutoClone,
	eg.Parallel(
		eg.Build(eg.DefaultModule()),
		eg.Build(c1.BuildFromFile(".eg/Containerfile")),
	),
	eg.Module(ctx, c1,
		eggolang.New().Timeout(10*time.Minute).AutoInstall(),
		eggolang.New().AutoTest(),
	),
	egdb.Cache, // keep last, saves the analytics db
)
```

`eg.Module(ctx, container, ops...)` re-runs the ops as a nested module inside that container, as user `egd`.

## Workload API (`runtime/wasi/`, stable)

- `eg`: `Perform` (ordered, stops at first error), `Sequential`, `Parallel` (aggregates errors), `When`, `WhenFn`, `Build`, `Module`, `Container(name).BuildFromFile|PullFrom`. An operation is `eg.OpFn`, `func(context.Context, eg.Op) error`.
- `shell`: runs commands on the container side. `shell.Op(shell.Newf("make %s", t))`. Builders: `.Timeout` (default 5m), `.Directory`, `.Environ(k, v)`, `.As(user)`, `.Privileged()` (root), `.Retry`, `.Lenient`, `.Silent`, `.Debug()`. `shell.Parallel(...)` runs several at once.
- `egenv`: `TTL()`, `RunID()`, `AccountID()`, `Draft()`, `Release()`, `DirectoryWorking|Cache|Workload|Temp()`.
- `eggit`: `AutoClone`, `Commitish`, `Change(path).Op(...)` with `NewCommit`, `Commit`, `Push`.
- `egmetrics.Record(ctx, name, payload)` for custom metrics. The `eg.` prefix is reserved.

## Helpers (`runtime/x/wasi/`)

Weaker stability promise than `runtime/wasi`, pin your eg version.

- Languages and builds: `eggolang` (`New().Tags().Flags().AutoInstall()|AutoCompile()|AutoTest()`), `egcargo`, `egyarn`, `egdart`, `egterraform`, `egccache`.
- Reporting: `egcoverage`, `egdb` (workload analytics, see `workload.analytics.md`), `egcache` (`Reset`).
- Release and packaging: `eggithub` (`Draft`, `Promote`, `Release`, `Upload`, `PullRequest`), `egtarball`, `egdebuild`, `egflatpak`, `egdmg`.
- Cloud and services: `eggcp`, `eggpg`, `egpostgresql`.
- Misc: `egllm`, `egnotif`, `egpool` (bounded concurrency), `egerrors`, `egenvx` (typed env readers: `New(def).From(keys...)`, `Exists(keys...)`, `Deferred(read, keys...)` to read at step time, e.g. `eg.WhenFn(egenvx.Deferred(egenvx.Exists, "TOKEN"), op)`).

## Containers

Prefer your own `.eg/Containerfile`. A custom image needs: the `eg` binary in `PATH`, an `egd` user with passwordless sudo in the root group, `sudo`, and DNS on 127.0.0.1:53. The easy route is `FROM ubuntu`, add `ppa:egdaemon/eg`, and install the `egworkload` package. Details: `custom.containers.md`.

## Running things

Local (no account needed):

```
eg compute local                      # default workload, in a podman container
eg compute local tests/envvars        # a named workload
eg compute local --debug              # keep the workspace for inspection
eg compute local --invalidate-cache   # drop the workload build cache
```

Useful `compute local` flags: `-e KEY=VAL`, `--envpath <file>`, `--secret <uri>`, `--ttl`, `--cores`, `--memory`, `--disk`, `--git-ref`, `--git-remote`, `--auto-patch` (apply changes the workload made to your worktree; `--no-auto-patch` prints a patch instead), `--gcp-auto`, `--github-token`.

Other local modes: `eg compute serve [module]` runs a long-lived service (`eg machine ssh` connects to a machine started with ssh enabled). `eg compute archive` builds a workload tarball for inspection.

Hosted service:

```
eg register                                  # once, creates account
eg login
eg compute upload --follow [module]          # compile, upload, stream logs
eg compute watch <id>
eg compute download logs|analytics <id>
eg compute cancel <id>
```

`upload` also takes `--git-ref`, `--git-remote`, `--envpath`, `--secret`, `--gcp-auto`. `eg compute pub ls` and `eg compute lan ls` list runners.

Identity and cloud: `eg repo whoami`, `eg me ls`, `eg gcp wif ...` (see `gcp.md`). Self-hosted runners: `eg actl bootstrap|authorize|uninstall` (see `self.hosted.md`).

Global flags: `-v` (repeat for more, `EG_LOGGING_VERBOSITY`), `--account-id`, `--profile-id`.

## Environment, secrets, directories

- A run does not inherit your environment. Pass what it needs with `-e`, `--envpath` or `--secret`.
- Secrets are named by URI: `chachasm://`, `file:`, `gcpsm://`, `awssm://`. eg injects them into the workload environment. Scheduled workloads may only use `gcpsm` and `awssm`.
- eg sets `EG_COMPUTE_RUN_ID`, `EG_COMPUTE_TTL`, `EG_COMPUTE_*_DIRECTORY`, `EG_GIT_HEAD_*`, `EG_WORKLOAD_DRAFT`, `EG_WORKLOAD_RELEASE` inside a run. Read them through `egenv`, not by hand.
- Inside a run: `/workload/eg` is the working copy of the repo, `/workload/.eg.cache` persists between runs, `.eg.workspace` is shared by nested modules, `os.TempDir()` is ephemeral. `/eg.mnt/.eg.runtime` belongs to eg. Use the `egenv.Directory*` functions rather than hardcoding paths.

## Sandbox gotchas

- The workload is wasip1: no `net.Dial` (use `wasinet`), no cgo, no direct host access.
- Anything touching the host (compilers, git, podman, package managers) goes through `shell` or an `x/wasi` helper.
- `shell` commands run as `egd`. Use `.Privileged()` for root, for example nested podman.
- The 5 minute default shell timeout is short for builds, set `.Timeout(...)`.
- Git: the default ref is `HEAD` for `compute local` and `main` for `compute upload`. Workloads using `eggit.AutoClone` need the right `--git-ref` and `--git-remote`.

## Debugging a failed run

1. Rerun with `-v` (up to `-vvvv`) and `--debug`, then inspect the kept workspace.
2. Add `.Debug()` to a shell command to see what it runs. It can echo environment values, so do not use it on runs that carry secrets.
3. Remote: `eg compute upload --follow`, `eg compute watch <id>`, `eg compute download logs <id>`.
4. Run metrics are in the workload analytics database, opened with `egdb.OpenVerify(ctx)`; builtin tables are `eg.metrics.*`.
