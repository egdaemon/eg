//go:build !wasm

package wasidialect

import (
	"unsafe"

	"github.com/james-lawrence/genieql/internal/wasix/ffierrors"
)

// Insert(n int, offset int, table string, conflict string, columns, projection, defaults []string) string
func _insertquery(
	n int64,
	offset int64,
	tableptr unsafe.Pointer, tablelen uint32,
	conflictptr unsafe.Pointer, conflictlen uint32,
	columnsptr unsafe.Pointer, columnslen uint32, columnssize uint32,
	projectionptr unsafe.Pointer, projectionlen uint32, projectionsize uint32,
	defaultsptr unsafe.Pointer, defaultslen uint32, defaultssize uint32,
	rlen unsafe.Pointer,
	rptr unsafe.Pointer,
) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}

// Select(table string, columns, predicates []string) string
func _selectquery(
	tableptr unsafe.Pointer, tablelen uint32,
	columnsptr unsafe.Pointer, columnslen uint32, columnssize uint32,
	predicatesptr unsafe.Pointer, predicateslen uint32, predicatessize uint32,
	rlen unsafe.Pointer,
	rptr unsafe.Pointer,
) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}

// Update(table string, columns, predicates, returning []string) string
func _updatequery(
	tableptr unsafe.Pointer, tablelen uint32,
	columnsptr unsafe.Pointer, columnslen uint32, columnssize uint32,
	predicatesptr unsafe.Pointer, predicateslen uint32, predicatessize uint32,
	returningptr unsafe.Pointer, returninglen uint32, returningsize uint32,
	rlen unsafe.Pointer,
	rptr unsafe.Pointer,
) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}

// Delete(table string, columns, predicates []string) string
func _deletequery(
	tableptr unsafe.Pointer, tablelen uint32,
	columnsptr unsafe.Pointer, columnslen uint32, columnssize uint32,
	predicatesptr unsafe.Pointer, predicateslen uint32, predicatessize uint32,
	rlen unsafe.Pointer,
	rptr unsafe.Pointer,
) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}

// QuotedString(s string) string
func _quotedString(sptr unsafe.Pointer, slen uint32, rlen unsafe.Pointer, rptr unsafe.Pointer) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}

// ColumnInformationForTable(table string) ([]genieql.ColumnInfo, error)
func _columninformationForTable(sptr unsafe.Pointer, slen uint32, rlen unsafe.Pointer, rptr unsafe.Pointer) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}

// ColumnInformationForQuery(query string) ([]genieql.ColumnInfo, error)
func _columninformationForQuery(sptr unsafe.Pointer, slen uint32, rlen unsafe.Pointer, rptr unsafe.Pointer) (errcode uint32) {
	return ffierrors.ErrNotImplemented
}
