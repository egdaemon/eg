# egenv v2

Plan for a v2 of the directory functions. Nothing here is implemented; v1 is untouched and keeps working.

## Goals
- Names that imply the lifecycle of the directory: `Directory{Scope}`.
- One shared implementation of the join/default logic instead of a copy per function.
- A relative-path function for each directory.
- Every function documented the same way: the absolute path (default and the env key that overrides it), then its lifecycle.

## Location
`runtime/wasi/egenv/v2`, a plain sub-package of the same module. v1 stays until callers have migrated.

## Names

Call sites are `egenv.XDirectory(` occurrences in the repo tree excluding vendor (includes a few in comments and tests).

| v1 | call sites | what it is | lifecycle | v2 |
|---|---|---|---|---|
| `EphemeralDirectory` | 24 | per-module `TMPDIR` (each module mounts its own) | one module's execution | `DirectoryTemp` |
| `WorkspaceDirectory` | 12 | `.eg.workspace`, shared persistent dir, e.g. git worktrees/patches, build output | all modules, one workload | `DirectoryRuntime` |
| `WorkingDirectory` | 38 | cloned repo checkout, initial cwd | one workload (content, not scratch) | `DirectoryWorking` |
| `CacheDirectory` | 50 | `.eg.cache`, best effort | survives across workloads | `DirectoryCache` |
| `RuntimeDirectory` | 5 | `.eg.runtime`, eg's own files and sockets | one workload | not in v2, see below |
| `WorkloadDirectory` | 1 | `/workload` root | one workload | removed from v1 already |
| `PathRelative*` | 15 | relative path helpers (Workspace only today) | n/a | `Relative`, `Relative{Scope}` |

Signature: plain `DirectoryX(paths ...string) string`. A `type Directory string` was rejected because Go cannot make it
implicitly assignable to `string`, which breaks interop at the bare call sites.

## eg's own runtime directory
`.eg.runtime` holds sockets, the module binary and the environ file; the host owns the contents and writing there can
break the runtime. v2 does not expose it. Callers use `egunsafe.RuntimeDirectory`, which reads
`EG_COMPUTE_RUNTIME_DIRECTORY` and falls back to the default mount.

Remaining v1 callers to move:
- `runtime/wasi/eggit/eggit.go` (`autoarchive`) and `eggit.autoarchive_test.go`: write/read the commit archive. This is a
  user-facing artifact with no in-tree host reader, so it is better served by the workspace directory. Moving it changes where
  `AutoArchive` writes (`.eg.runtime` -> `.eg.workspace`); needs a decision before doing it.
- `cmd/eg/compute/.bootstrap.c8s/main.go`: reads `workspace/Containerfile` placed by the host; use `egunsafe.RuntimeDirectory`.
- `.eg/tests/envvars/main.go`: inspects eg's own dir; use `egunsafe.RuntimeDirectory`.
- `www/.eg/docs/docs.go`: replace with `egunsafe.RuntimeDirectory("eg")`.

## Relative paths
- `Relative(root, path string) string`: v2 of `PathRelative`. Relative paths and paths outside root are returned as is.
- `Relative{Scope}(path string) string` for each directory, delegating to `Relative`.
- Replaces the hand-rolled `PathRelative(egenv.WorkingDirectory(), ...)` in `runtime/x/wasi/egautogen/egautogen.go`.

## Layout
- `dirs.go`: shared funcs (private `dir(fallback, key, paths)` helper, `Relative`).
- `dirs.{name}.go`: the functions specific to that directory (`dirs.cache.go`, `dirs.working.go`, `dirs.runtime.go`,
  `dirs.temp.go`).
- One test file per function, mirroring the split (`dirs.cache_test.go`, ...).

## Behavior
Keep the v1 defaults and fallbacks (temp stays `TMPDIR` based, `CACHE_DIRECTORY` stays an accepted key for cache) unless
decided otherwise.

## Docs convention
For every function, in this order:
1. the absolute path of the directory (default and the env key that overrides it),
2. its lifecycle,

then briefly that paths are joined, and an `e.g.)` line.

## Tests convention
Package `egenv_test`, style of `egenv.workspace_test.go`: `t.Setenv`, `require`, no helper funcs, inline setup. Per function:
env set, env unset (default), multiple path args, no args. Cache: `CACHE_DIRECTORY` fallback and precedence.

## Deprecating v1
Go convention is a paragraph starting with `Deprecated:` in a doc comment. Put one in the package comment of
`runtime/wasi/egenv` (`// Deprecated: use github.com/egdaemon/eg/runtime/wasi/egenv/v2.`) so gopls strikes through and
staticcheck (SA1019) flags every import. Also add per-symbol `Deprecated:` on each v1 function with the precise replacement
(e.g. `// Deprecated: use v2.DirectoryTemp.`).

Caveat: this flags every v1 call site (and fails any lint run using staticcheck) until they migrate, so do it as its own
stage, at the start or the end of the caller migration.

## Order of work
1. Build v2 (code, docs, tests per the conventions above).
2. Migrate callers package by package, one stage each.
3. Deprecate v1.
4. v1 docs and tests (typos such as "joined joined", missing `WorkingDirectory` description, tests for the directory
   functions) can land at any point before 3.

## Open questions
- Is `v2` as a sub-package the right shape, or should v1 evolve in place with deprecations?
- Confirm a `Relative{Scope}` function per directory (6 functions) alongside `Relative(root, path)`.
- Move the eggit autoarchive to the workspace directory?
