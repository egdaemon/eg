package eg

// genieql code generation. every package generating against the cli database shares the
// same migrations (cmd/migrations) and database (eg.db), so it is set up once, then each package is generated.
//go:generate genieql duckdb --database=eg.db cmd/migrations
//go:generate genieql bootstrap --queryer=sqlx.Queryer --driver=github.com/marcboeker/go-duckdb duckdb://localhost/eg.db
//go:generate genieql auto graph -o genieql.gen.go
