//go:build wasm

package wasidialect

import (
	"unsafe"
)

//go:wasmimport env genieql/dialect.Insert
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
) (errcode uint32)

//go:wasmimport env genieql/dialect.Select
func _selectquery(
	tableptr unsafe.Pointer, tablelen uint32,
	columnsptr unsafe.Pointer, columnslen uint32, columnssize uint32,
	predicatesptr unsafe.Pointer, predicateslen uint32, predicatessize uint32,
	rlen unsafe.Pointer,
	rptr unsafe.Pointer,
) (errcode uint32)

//go:wasmimport env genieql/dialect.Update
func _updatequery(
	tableptr unsafe.Pointer, tablelen uint32,
	columnsptr unsafe.Pointer, columnslen uint32, columnssize uint32,
	predicatesptr unsafe.Pointer, predicateslen uint32, predicatessize uint32,
	returningptr unsafe.Pointer, returninglen uint32, returningsize uint32,
	rlen unsafe.Pointer,
	rptr unsafe.Pointer,
) (errcode uint32)

//go:wasmimport env genieql/dialect.Delete
func _deletequery(
	tableptr unsafe.Pointer, tablelen uint32,
	columnsptr unsafe.Pointer, columnslen uint32, columnssize uint32,
	predicatesptr unsafe.Pointer, predicateslen uint32, predicatessize uint32,
	rlen unsafe.Pointer,
	rptr unsafe.Pointer,
) (errcode uint32)

//go:wasmimport env genieql/dialect.QuotedString
func _quotedString(ptr unsafe.Pointer, len uint32, rlen unsafe.Pointer, rptr unsafe.Pointer) (errcode uint32)

//go:wasmimport env genieql/dialect.ColumnInformationForTable
func _columninformationForTable(sptr unsafe.Pointer, slen uint32, rlen unsafe.Pointer, rptr unsafe.Pointer) (errcode uint32)

//go:wasmimport env genieql/dialect.ColumnInformationForQuery
func _columninformationForQuery(sptr unsafe.Pointer, slen uint32, rlen unsafe.Pointer, rptr unsafe.Pointer) (errcode uint32)
