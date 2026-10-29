package coverage

import (
	"context"
	"errors"
	"io"

	"github.com/Masterminds/squirrel"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/sqlx"
)

// Query runs b against q. q is not closed, it belongs to the caller.
func Query(q sqlx.Queryer, b squirrel.SelectBuilder) iterx.Seq[Report] {
	return iterx.New(func(ctx context.Context, yield func(Report) bool) error {
		rows := sqlx.Scan(ReportSearch(ctx, sqlx.Debug(q), b))
		for r := range rows.Iter() {
			if !yield(r) {
				break
			}
		}

		return errorsx.Wrap(rows.Err(), "unable to query coverage")
	})
}

// From runs b against the database open returns, e.g.) egdb.Open. the database is opened when iteration starts and
// closed when it ends, including when it stops early.
func From[D interface {
	sqlx.Queryer
	io.Closer
}](open func(context.Context) (D, error), b squirrel.SelectBuilder) iterx.Seq[Report] {
	return iterx.New(func(ctx context.Context, yield func(Report) bool) (err error) {
		db, err := open(ctx)
		if err != nil {
			return err
		}
		defer func() {
			err = errors.Join(err, db.Close())
		}()

		reports := Query(db, b)
		for r := range reports.Each(ctx) {
			if !yield(r) {
				break
			}
		}

		return reports.Err()
	})
}
