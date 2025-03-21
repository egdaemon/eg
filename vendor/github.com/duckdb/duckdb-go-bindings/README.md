# duckdb-go-bindings

🚧 WORK IN PROGRESS 🚧

This repository wraps DuckDB's C API calls in Go native types and functions.

The main module (`github.com/duckdb/duckdb-go-bindings`) does not link any pre-built static library.

TODO: example on static linking
TODO: example on dynamic linking

There are also a few pre-built static libraries for different OS + architecture combinations.
Here's a list:
- `github.com/duckdb/duckdb-go-bindings/`...
  - `darwin-amd64`
  - `darwin-arm64`
  - `linux-amd64`
  - `linux-arm64`
  - `windows-amd64`

The first official release of this module will contain DuckDB's v1.2.0 release.
