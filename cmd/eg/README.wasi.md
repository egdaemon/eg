# eg wasi

hidden commands for working with standalone wasi modules.

## run a module with coreutils available

```
eg wasi run --coreutils=<path> <module>
```

- `<module>`: the eg module to compile and run, same as `eg wasi run <module>`.
- `--directory`: root directory of the repository, defaults to the git directory.
- `--coreutils=<path>`: path to a coreutils reactor module (wasm32-wasip1). It is loaded from where it is,
  nothing is copied or embedded. When set, the module can call the `unsafecoreutils` package
  (`runtime/x/wasi/unsafecoreutils`), e.g. `unsafecoreutils.Ls(ctx, "-la", "/")`.

The utilities read and write the stdio of the module that calls them (here, the process's stdin, stdout and stderr)
and see the same directories as the module (the well known workspace directories, same layout as `interp`).

### building the coreutils module

from the coreutils repository (see its `README.wasi.md`):

```
cargo rustc --lib --crate-type cdylib --release --target wasm32-wasip1 --no-default-features --features "feat_wasm feat_wasm_reactor"
```

the module is written to `target/wasm32-wasip1/release/coreutils.wasm`.

### regenerating the shims

`runtime/x/wasi/unsafecoreutils` has one function per utility, generated from the module's `uu_*` exports:

```
EG_COREUTILS_WASM=<path> go generate ./runtime/x/wasi/unsafecoreutils
```

### notes

- utilities run on a single shared instance of the module, calls are serialized. A call that blocks (waiting on
  stdin, `sleep`) blocks every other call until it finishes.
- the module's directories are fixed paths (`/eg.mnt/...`, `/workload/...`), so the command needs them to exist
  and be writable.

## inspect a module

```
eg wasi debug <path>
```

prints the functions a core wasi module exports and imports (to stderr). Components (wasm32-wasip2) are not
supported, build the module for wasm32-wasip1.
