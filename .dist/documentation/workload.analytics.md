---
created: 2026-10-27T05:00:00.000Z
updated: 2026-10-30T05:00:00.000Z
title: "Workload Analytics"
index: 3
---

## Workload Analytics

every run has an analytics database, a DuckDB database the runner records into and serves to your workload. DuckDB is an embedded database: one process opens the file and owns it. eg runs your workload as a sandboxed wasm module in a separate process from the runner, so the runner serves its analytics database to the workload over two unix sockets using a custom duckdb and a pg wire protocol. the client(s) is a regular `database/sql` driver, so anything that works with `*sql.DB` works with it. magic!

| package                                  | where it runs                     | purpose                                            |
| ---------------------------------------- | --------------------------------- | -------------------------------------------------- |
| `github.com/egdaemon/eg/duckproxy`       | workload (wasm) or any Go process | `database/sql` driver, no cgo                      |
| `github.com/egdaemon/eg/duckproxyserver` | runner, requires duckdb-go (cgo)  | serves a `*sql.DB` on a unix socket                |
| `github.com/egdaemon/eg/duckpgwire`      | optional                          | postgres wire frontend over a duckproxy connection |

## builtin analytics and conventions

eg provides a few builtin analytic tables. they live in the database next to any tables your workload creates.
Its important to note that the table prefix `eg.` is reserved for builtin platform use. if you use it and it clashes with future changes thats on you, not eg.

| table                  | holds                                                                                                                                                                 |
| ---------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `eg.metrics.custom`    | metrics recorded by workloads, with a json payload.                                                                                                                   |
| `eg.metrics.operation` | timings of the operations a run performed.                                                                                                                            |
| `eg.metrics.cpu`       | the load of the machine, recorded by the runner while it is running. the runner creates this table itself, it is not part of the migrations below.                    |
| `eg.metrics.coverage`  | test coverage per file, and per function when `fnname` is set: `statements` and `branches` as percentages, `hits`, and `generated` for files that are generated code. |

### recording coverage

`egcoverage.Report` records coverage from a source such as `eggolang.Coverage()` or `eglcov.Coverage(dir)` into `eg.metrics.coverage`. mark generated files when you record them, e.g., `eggolang.Coverage().MarkGeneratedExt(".gen.go", ".pb.go")`, and tools that read the table, such as the test generator, can skip them.

### schema and migrations

the other tables above are created by migrations that ship inside eg. the runner applies them when it creates the analytics database. a workload can be newer than the runner that hosts it, e.g., a runner that hasn't been upgraded yet, and would then apply migrations that runner doesn't have.

`egdb.OpenMigrate(ctx)` is `egdb.OpenVerify` followed by the same migrations. migrations that were already applied are skipped, so it is safe to call when the runner is current. operations that write eg's own tables, such as `egcoverage.Report`, use it before writing.

set `EG_COMPUTE_GUEST_MIGRATIONS_ENABLED=false` in the environment of the runner to turn this off, which makes `OpenMigrate` behave like `OpenVerify`. it is enabled by default.

## from a workload

```go
import "github.com/egdaemon/eg/runtime/x/wasi/egdb"

db, err := egdb.OpenVerify(ctx)
if err != nil {
	return err
}
defer db.Close()

var n int
if err := db.QueryRowContext(ctx, "SELECT count(*) FROM 'eg.metrics.cpu'").Scan(&n); err != nil {
	return err
}
```

`egdb.OpenVerify` dials the runner's `analytics.socket` and pings before returning, so a missing runner is reported immediately. `egdb.Open` returns the same `*sql.DB` without the ping and without an error, connecting when it is first used. you must close the result of either. the database is shared with the runner and with other operations in your workload keep this in mind just like you would any production database.

### other databases

- `egdb.Latest(ctx, egdb.WithWorkload(name))` locates the analytics database of the last successful run of a workload.
- `egdb.OpenFile(ctx, path)` opens a database file read only, e.g., the one `Latest` returned. it is served by the runner separately from the analytics database. close it when done.
- `egdb.Cache` is an operation that saves a copy of the analytics database into the cache directory so `Latest` can find it later. place it last in your workload.

## outside of a workload

use the driver directly with the socket path as the dsn.

```go
import _ "github.com/egdaemon/eg/duckproxy"

db, err := sql.Open("duckproxy", "/path/to/analytics.socket")
```

inside a wasm guest a plain `net.Dial` is unavailable, so provide your own dial function:

```go
db := sql.OpenDB(duckproxy.NewConnector(path, wasinet.DialContext))
```

## other languages

the runner also serves the database over the postgres wire protocol, so any postgres client in any language can use it. the socket is named `.s.PGSQL.5432` in the workload's runtime directory, which is the name libpq style clients look for: set the host to the directory, which is in the `EG_COMPUTE_RUNTIME_DIRECTORY` environment variable.

```python
import os
import psycopg

with psycopg.connect(host=os.environ["EG_COMPUTE_RUNTIME_DIRECTORY"], dbname="eg") as conn:
    print(conn.execute("SELECT count(*) FROM 'eg.metrics.cpu'").fetchone())
```

```typescript
import { Client } from "pg"

const client = new Client({ host: process.env.EG_COMPUTE_RUNTIME_DIRECTORY, database: "eg" })
await client.connect()
const { rows } = await client.query("SELECT count(*) FROM 'eg.metrics.cpu'")
await client.end()
```
