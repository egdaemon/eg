# duckdb-go-bindings

![Tests status](https://github.com/duckdb/duckdb-go-bindings/actions/workflows/run_tests.yml/badge.svg)

This repository wraps DuckDB's C API calls in Go native types and functions.

Tested Go versions: 1.23, 1.24.

```diff
+ Some type aliases and function wrappers are still missing.
```

## Releases

This module's *first* official release contains DuckDB's v1.2.0 release.

| duckdb version | main module | darwin amd | darwin arm | linux amd | linux arm | windows amd |
|----------------|-------------|------------|------------|-----------|-----------|-------------|
| v1.2.2         | v0.1.14     | v0.1.9     | v0.1.9     | v0.1.9    | v0.1.9    | v0.1.9      |
| v1.2.1         | v0.1.13     | v0.1.8     | v0.1.8     | v0.1.8    | v0.1.8    | v0.1.8      |
| v1.2.0         | v0.1.10     | v0.1.5     | v0.1.5     | v0.1.5    | v0.1.5    | v0.1.5      |

The main module (`github.com/duckdb/duckdb-go-bindings`) does not link any pre-built static library.

## Using a pre-built static library

A few pre-built static libraries exist for different OS + architecture combinations.
You can import these into your projects without providing additional build flags.
`CGO` must be enabled, and your system needs a compiler available.

Here's a list:
- `github.com/duckdb/duckdb-go-bindings/`...
  - `darwin-amd64`
  - `darwin-arm64`
  - `linux-amd64`
  - `linux-arm64`
  - `windows-amd64`

## Static linking

Note that the lib(s) name must match the name provided in the `CGO_LDFLAGS`.

On Darwin. 
```
CGO_ENABLED=1 CPPFLAGS="-DDUCKDB_STATIC_BUILD" CGO_LDFLAGS="-lduckdb -lc++ -L/path/to/lib" go build -tags=duckdb_use_static_lib
```

On Linux.
```
CGO_ENABLED=1 CPPFLAGS="-DDUCKDB_STATIC_BUILD" CGO_LDFLAGS="-lduckdb -lstdc++ -lm -ldl -L/path/to/lib" go build -tags=duckdb_use_static_lib
```

On Windows.
```
CGO_ENABLED=1 CPPFLAGS="-DDUCKDB_STATIC_BUILD" CGO_LDFLAGS="-lduckdb -lws2_32 -lwsock32 -lrstrtmgr -lstdc++ -lm --static -L/path/to/lib" go build -tags=duckdb_use_static_lib
```

## Dynamic linking

On Darwin.
```
CGO_ENABLED=1 CGO_LDFLAGS="-lduckdb -L/path/to/dir" DYLD_LIBRARY_PATH=/path/to/dir go build -tags=duckdb_use_lib
```

On Linux.
```
CGO_ENABLED=1 CGO_LDFLAGS="-lduckdb -L/path/to/dir" LD_LIBRARY_PATH=/path/to/dir go build -tags=duckdb_use_lib
```


