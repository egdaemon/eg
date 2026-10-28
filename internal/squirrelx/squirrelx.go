// Package squirrelx provides helpers for building queries with squirrel.
package squirrelx

import (
	"github.com/Masterminds/squirrel"
)

// PSQL statement builder using dollar placeholders, as used by postgresql and duckdb.
var PSQL = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

// Noop is a predicate that adds nothing to a query, for filters that were not provided.
type Noop struct{}

func (t Noop) ToSql() (sql string, args []interface{}, err error) {
	return "", nil, nil
}
