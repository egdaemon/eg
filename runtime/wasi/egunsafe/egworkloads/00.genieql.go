//go:build genieql.generate

package egworkloads

import (
	"context"

	"github.com/egdaemon/eg/internal/sqlx"
	genieql "github.com/james-lawrence/genieql/ginterp"
)

func Discovered(gql genieql.Structure) {
	gql.From(gql.Table("eg_workloads_discovered"))
}

func DiscoveredScanner(gql genieql.Scanner, pattern func(w Discovered)) {}

func DiscoveredInsert(
	gql genieql.Insert,
	pattern func(ctx context.Context, q sqlx.Queryer, w Discovered) NewDiscoveredScannerStaticRow,
) {
	gql.Into("eg_workloads_discovered")
}

func DiscoveredLookup(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer) NewDiscoveredScannerStatic,
) {
	gql = gql.Query(`SELECT ` + DiscoveredScannerStaticColumns + ` FROM eg_workloads_discovered ORDER BY ts DESC`)
}
