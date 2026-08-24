package wasidialect

import (
	"encoding/json"
	"errors"
	"fmt"
	"unsafe"

	"github.com/james-lawrence/genieql"
	"github.com/james-lawrence/genieql/dialects"
	"github.com/james-lawrence/genieql/internal/bytesx"
	"github.com/james-lawrence/genieql/internal/errorsx"
	"github.com/james-lawrence/genieql/internal/postgresql"
	"github.com/james-lawrence/genieql/internal/wasix/ffierrors"
	"github.com/james-lawrence/genieql/internal/wasix/ffiguest"
	"golang.org/x/text/transform"
)

// Dialect constant representing the dialect name.
const Dialect = "wasi"

func init() {
	errorsx.MaybePanic(dialects.Register(Dialect, dialectFactory{}))
}

type dialectFactory struct{}

func (t dialectFactory) Connect(config genieql.Configuration) (_ genieql.Dialect, err error) {
	return New(), nil
}

func New() dialect {
	return dialect{
		columntrans:         postgresql.NewColumnValueTransformer,
		columnnametransform: postgresql.NewColumnNameTransformer,
	}
}

type dialect struct {
	columntrans         func() genieql.ColumnTransformer
	columnnametransform func(transforms ...transform.Transformer) genieql.ColumnTransformer
}

func (t dialect) Insert(n int, offset int, table string, conflict string, columns, projection, defaults []string) string {
	var (
		rs = make([]byte, 0, 2*bytesx.MiB)
	)
	tableptr, tablelen := ffiguest.String(table)
	conflictptr, conflictlen := ffiguest.String(conflict)
	columnsptr, columnslen, columnssize := ffiguest.StringArray(columns...)
	projectionptr, projectionlen, projectionsize := ffiguest.StringArray(projection...)
	defaultsptr, defaultslen, defaultssize := ffiguest.StringArray(defaults...)

	_, rptr, rlen := ffiguest.ByteBuffer(rs)

	errorsx.MaybePanic(ffierrors.Error(
		_insertquery(
			int64(n),
			int64(offset),
			tableptr, tablelen,
			conflictptr, conflictlen,
			columnsptr, columnslen, columnssize,
			projectionptr, projectionlen, projectionsize,
			defaultsptr, defaultslen, defaultssize,
			unsafe.Pointer(&rlen),
			rptr,
		),
		errors.New("unable generate insert"),
	))
	decoded := unsafe.String(unsafe.SliceData(rs), rlen)

	return decoded
}

func (t dialect) Select(table string, columns, predicates []string) string {
	var (
		rs = make([]byte, 0, 2*bytesx.MiB)
	)
	tableptr, tablelen := ffiguest.String(table)
	columnsptr, columnslen, columnssize := ffiguest.StringArray(columns...)
	predicatesptr, predicateslen, predicatessize := ffiguest.StringArray(predicates...)

	_, rptr, rlen := ffiguest.ByteBuffer(rs)

	errorsx.MaybePanic(ffierrors.Error(
		_selectquery(
			tableptr, tablelen,
			columnsptr, columnslen, columnssize,
			predicatesptr, predicateslen, predicatessize,
			unsafe.Pointer(&rlen),
			rptr,
		),
		errors.New("unable generate select"),
	))
	decoded := unsafe.String(unsafe.SliceData(rs), rlen)

	return decoded
}

func (t dialect) Update(table string, columns, predicates, returning []string) string {
	var (
		rs = make([]byte, 0, 2*bytesx.MiB)
	)
	tableptr, tablelen := ffiguest.String(table)
	columnsptr, columnslen, columnssize := ffiguest.StringArray(columns...)
	predicatesptr, predicateslen, predicatessize := ffiguest.StringArray(predicates...)
	returningptr, returninglen, returningsize := ffiguest.StringArray(returning...)

	_, rptr, rlen := ffiguest.ByteBuffer(rs)

	errorsx.MaybePanic(ffierrors.Error(
		_updatequery(
			tableptr, tablelen,
			columnsptr, columnslen, columnssize,
			predicatesptr, predicateslen, predicatessize,
			returningptr, returninglen, returningsize,
			unsafe.Pointer(&rlen),
			rptr,
		),
		errors.New("unable generate update"),
	))
	decoded := unsafe.String(unsafe.SliceData(rs), rlen)

	return decoded
}

func (t dialect) Delete(table string, columns, predicates []string) string {
	var (
		rs = make([]byte, 0, 2*bytesx.MiB)
	)
	tableptr, tablelen := ffiguest.String(table)
	columnsptr, columnslen, columnssize := ffiguest.StringArray(columns...)
	predicatesptr, predicateslen, predicatessize := ffiguest.StringArray(predicates...)

	_, rptr, rlen := ffiguest.ByteBuffer(rs)

	errorsx.MaybePanic(ffierrors.Error(
		_deletequery(
			tableptr, tablelen,
			columnsptr, columnslen, columnssize,
			predicatesptr, predicateslen, predicatessize,
			unsafe.Pointer(&rlen),
			rptr,
		),
		errors.New("unable generate delete"),
	))
	decoded := unsafe.String(unsafe.SliceData(rs), rlen)

	return decoded
}

func (t dialect) ColumnValueTransformer() genieql.ColumnTransformer {
	return t.columntrans()
}

func (t dialect) ColumnNameTransformer(opts ...transform.Transformer) genieql.ColumnTransformer {
	return t.columnnametransform(opts...)
}

func (t dialect) ColumnInformationForTable(d genieql.Driver, table string) (res []genieql.ColumnInfo, err error) {
	var (
		rs = make([]byte, 0, 2*bytesx.MiB)
	)

	sptr, slen := ffiguest.String(table)
	_, rptr, rlen := ffiguest.ByteBuffer(rs)

	err = ffierrors.Error(
		_columninformationForTable(sptr, slen, unsafe.Pointer(&rlen), rptr),
		fmt.Errorf("unable to query column information for table: %s", table),
	)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(ffiguest.ByteBufferRead(rptr, rlen), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (t dialect) ColumnInformationForQuery(d genieql.Driver, query string) (res []genieql.ColumnInfo, err error) {
	var (
		rs = make([]byte, 0, 2*bytesx.MiB)
	)

	sptr, slen := ffiguest.String(query)
	_, rptr, rlen := ffiguest.ByteBuffer(rs)

	err = ffierrors.Error(
		_columninformationForQuery(sptr, slen, unsafe.Pointer(&rlen), rptr),
		fmt.Errorf("unable to query column information for query: %s", query),
	)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal(ffiguest.ByteBufferRead(rptr, rlen), &res); err != nil {
		return nil, err
	}

	return res, nil
}

func (t dialect) QuotedString(s string) string {
	var (
		rs = make([]byte, 0, 1024)
	)
	sptr, slen := ffiguest.String(s)
	_, rptr, rlen := ffiguest.ByteBuffer(rs)

	errorsx.MaybePanic(ffierrors.Error(
		_quotedString(sptr, slen, unsafe.Pointer(&rlen), rptr),
		errors.New("unable to quote string"),
	))
	decoded := unsafe.String(unsafe.SliceData(rs), rlen)

	return decoded
}
