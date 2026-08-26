package egworkloads

//go:generate genieql duckdb --database=egworkloads.db ./migrations
//go:generate genieql bootstrap --queryer=sqlx.Queryer --driver=github.com/marcboeker/go-duckdb duckdb://localhost/egworkloads.db
//go:generate genieql auto graph -o genieql.gen.go
