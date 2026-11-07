# eg wasi

hidden commands for working with standalone wasi modules.

## run a module

```
eg wasi run <module>
```

- `<module>`: the eg module to compile and run.
- `--directory`: root directory of the repository, defaults to the git directory.

The modules in the wasi module directory (`wasi.mod.d`, see `interp/runtime/wasi/wasimod`) are loaded before the
module runs, the same as `interp`. Each `*.wasi` file is instantiated under its file name, so the module binds to its
exports with `go:wasmimport`. Install the coreutils reactor there as `egcoreutils.wasi` for `fficoreutils.Cmd`.

The modules see the same directories as the module (the well known workspace directories, same layout as `interp`).

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

- the module's directories are fixed paths (`/eg.mnt/...`, `/workload/...`), so the command needs them to exist
  and be writable.

## inspect a module

```
eg wasi debug <path>
```

prints the functions a core wasi module exports and imports (to stderr). Components (wasm32-wasip2) are not
supported, build the module for wasm32-wasip1.
