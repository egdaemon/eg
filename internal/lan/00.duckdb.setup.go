package lan

//go:generate genieql duckdb --database=lan.db ./migrations
//go:generate genieql bootstrap --queryer=sqlx.Queryer --driver=github.com/marcboeker/go-duckdb duckdb://localhost/lan.db
//go:generate genieql auto graph -o genieql.gen.go
