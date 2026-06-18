//go:build genieql.generate
// +build genieql.generate

package coverage

import (
	"context"

	"github.com/egdaemon/eg/internal/sqlx"
	genieql "github.com/james-lawrence/genieql/ginterp"
)

func Report(gql genieql.Structure) {
	gql.From(
		gql.Table("'eg.metrics.coverage'"),
	)
}

func ReportScanner(gql genieql.Scanner, pattern func(i Report)) {
	gql.ColumnNamePrefix("")
}

func ReportBatchInsert(
	gql genieql.InsertBatch,
	pattern func(ctx context.Context, q sqlx.Queryer, p Report) NewReportScannerStatic,
) {
	gql.Into("eg.metrics.coverage").Default("id").Batch(128)
}

func Worst(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, n int) NewReportScannerStatic,
) {
	gql = gql.Query(`SELECT ` + ReportScannerStaticColumns + ` FROM 'eg.metrics.coverage' WHERE fnname != '' ORDER BY hits ASC LIMIT {n}`)
}

func Sample(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, n int) NewReportScannerStatic,
) {
	gql = gql.Query(`SELECT ` + ReportScannerStaticColumns + ` FROM 'eg.metrics.coverage' WHERE fnname != '' ORDER BY random() LIMIT {n}`)
}
