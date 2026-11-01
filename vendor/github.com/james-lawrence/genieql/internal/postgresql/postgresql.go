package postgresql

import (
	"database/sql"
	"fmt"
	"go/ast"
	"go/types"
	"log"

	"github.com/davecgh/go-spew/spew"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/text/transform"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/astutil"
	"github.com/james-lawrence/genieql/columninfo"
	"github.com/james-lawrence/genieql/dialects"
	"github.com/james-lawrence/genieql/internal/debugx"
	"github.com/james-lawrence/genieql/internal/errorsx"
	"github.com/james-lawrence/genieql/internal/postgresql/internal"
	"github.com/james-lawrence/genieql/internal/transformx"
)

// Dialect constant representing the dialect name.
const Dialect = "postgres"

// NewDialect creates a postgresql Dialect from the queryer
func NewDialect(q *sql.DB) genieql.Dialect {
	return dialectImplementation{db: q}
}

func init() {
	errorsx.MaybePanic(dialects.Register(Dialect, dialectFactory{}))
}

type queryer interface {
	Query(string, ...interface{}) (*sql.Rows, error)
}

type dialectFactory struct{}

func (t dialectFactory) Connect(config genieql.Configuration) (_ genieql.Dialect, err error) {
	pcfg, err := pgx.ParseConfig(config.ConnectionURL)
	if err != nil {
		return nil, errorsx.Wrapf(err, "unable to parse postgresql connection string: %s", config.ConnectionURL)
	}

	slib := stdlib.OpenDB(*pcfg)
	if err = slib.Ping(); err != nil {
		return nil, errorsx.Wrapf(err, "unable to connect to database: %s", config.ConnectionURL)
	}

	return dialectImplementation{db: slib}, nil
}

func NewColumnValueTransformer() genieql.ColumnTransformer {
	return &ColumnValueTransformer{}
}

func NewColumnNameTransformer(transforms ...transform.Transformer) genieql.ColumnTransformer {
	return columninfo.NewNameTransformer(
		transformx.Wrap("\""),
		transform.Chain(transforms...),
	)
}

type dialectImplementation struct {
	db *sql.DB
}

func (t dialectImplementation) Insert(n int, offset int, table, conflict string, columns, projection, defaults []string) string {
	return Insert(n, offset, table, conflict, columns, projection, defaults)
}

func (t dialectImplementation) Select(table string, columns, predicates []string) string {
	return Select(table, columns, predicates)
}

func (t dialectImplementation) Update(table string, columns, predicates, returning []string) string {
	return Update(table, columns, predicates, returning)
}

func (t dialectImplementation) Delete(table string, columns, predicates []string) string {
	return Delete(table, columns, predicates)
}

func (t dialectImplementation) ColumnValueTransformer() genieql.ColumnTransformer {
	return NewColumnValueTransformer()
}

func (t dialectImplementation) ColumnNameTransformer(transforms ...transform.Transformer) genieql.ColumnTransformer {
	return NewColumnNameTransformer(transforms...)
}

func (t dialectImplementation) ColumnInformationForTable(d genieql.Driver, table string) ([]genieql.ColumnInfo, error) {
	const columnInformationQuery = `SELECT a.attname, a.atttypid, format_type(a.atttypid, NULL), NOT a.attnotnull AS nullable, COALESCE(a.attnum = ANY(i.indkey), 'f') AND COALESCE(i.indisprimary, 'f') AS isprimary FROM pg_index i RIGHT OUTER JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey) AND i.indisprimary = 't' WHERE a.attrelid = ($1)::regclass AND a.attnum > 0 AND a.attisdropped = 'f'`
	return columnInformation(d, t.db, columnInformationQuery, table)
}

func (t dialectImplementation) ColumnInformationForQuery(d genieql.Driver, query string) ([]genieql.ColumnInfo, error) {
	const columnInformationQuery = `SELECT a.attname, a.atttypid, format_type(a.atttypid, NULL), 'f' AS nullable, 'f' AS isprimary FROM pg_index i RIGHT OUTER JOIN pg_attribute a ON a.attrelid = i.indrelid WHERE a.attrelid = ($1)::regclass AND a.attnum > 0`
	const table = "genieql_query_columns_table"

	tx, err := t.db.Begin()
	if err != nil {
		return nil, errorsx.Wrap(err, "failure to start transaction")
	}
	defer tx.Rollback()

	q := fmt.Sprintf("CREATE TABLE %s AS (%s LIMIT 1)", table, query)
	if _, err = tx.Exec(q); err != nil {
		return nil, errorsx.Wrapf(err, "failure to execute %s", q)
	}

	return columnInformation(d, tx, columnInformationQuery, table)
}

func (t dialectImplementation) QuotedString(s string) string {
	return quotedString(s)
}

func columnInformation(d genieql.Driver, q queryer, query, table string) ([]genieql.ColumnInfo, error) {
	var (
		err     error
		rows    *sql.Rows
		columns []genieql.ColumnInfo
	)

	if rows, err = q.Query(query, table); err != nil {
		return nil, errorsx.Wrapf(err, "failed to query column information: %s, %s", query, table)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			columndef genieql.ColumnDefinition
			oid       int
			tname     string
			expr      ast.Expr
			primary   bool
			nullable  bool
			name      string
		)

		if err = rows.Scan(&name, &oid, &tname, &nullable, &primary); err != nil {
			return nil, errorsx.Wrapf(err, "error scanning column information for table (%s): %s", table, query)
		}

		expr = internal.OIDToType(oid)
		if expr == nil {
			log.Println("nonstandard column type", name, "unknown type identifier", oid, "falling back to type name", tname)
			expr = astutil.Expr(tname)
		}

		if columndef, err = d.LookupType(types.ExprString(expr)); err != nil {
			log.Println("skipping column", name, "driver missing type", types.ExprString(expr), "please open an issue")
			continue
		}

		switch columndef.Native {
		case "[]byte":
			columndef.Nullable = false
		default:
			columndef.Nullable = nullable
		}

		columndef.PrimaryKey = primary

		debugx.Println("found column", name, types.ExprString(expr), spew.Sdump(columndef))

		columns = append(columns, genieql.ColumnInfo{
			Name:       name,
			Definition: columndef,
		})
	}

	columns = genieql.SortColumnInfo(columns)(genieql.ByName)

	return columns, errorsx.Wrap(rows.Err(), "error retrieving column information")
}
