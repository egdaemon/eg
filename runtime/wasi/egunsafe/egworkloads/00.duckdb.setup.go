package egworkloads

//go:generate genieql duckdb --database=egworkloads.db ./migrations
//go:generate genieql bootstrap --output-file=egworkloads.config --queryer=sqlx.Queryer --driver=github.com/marcboeker/go-duckdb duckdb://localhost/egworkloads.db
//go:generate genieql auto graph --config=egworkloads.config -o genieql.gen.go
