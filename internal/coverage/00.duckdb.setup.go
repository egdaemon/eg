package coverage

//go:generate genieql duckdb --database=coverage.db ./.migrations
//go:generate genieql bootstrap --queryer=sqlx.Queryer --driver=github.com/marcboeker/go-duckdb duckdb://localhost/coverage.db
//go:generate genieql auto graph -o genieql.gen.go
