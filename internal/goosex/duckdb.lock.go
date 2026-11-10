package goosex

import (
	"context"
	"database/sql"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/gofrs/uuid/v5"
	"github.com/pressly/goose/v3/lock"
)

const (
	// the table holding the lock, created on first use like goose's version table.
	locktable = "goose_lock"
	// the one row of the lock table, goose's default lock id.
	lockid = lock.DefaultLockID
	// how long a holder owns the lock. migrations are short, a holder that died is taken over once its lease ends.
	locklease = 30 * time.Second
	// how long to wait to acquire, or release, the lock. longer than the lease so a dead holder is always taken over.
	lockwait = 2 * locklease
	// how often a held lock, or a conflict with a concurrent attempt, is retried.
	lockpoll = 25 * time.Millisecond
)

// duckdbLocker is a goose lock.SessionLocker for duckdb, which has no advisory locks, LOCK TABLE or SELECT FOR UPDATE.
// duckdb is optimistic: concurrent writers to the same rows or catalog entries don't wait, the loser is aborted
// with a write-write conflict. so the lock is a row in a table with a lease: acquiring is an upsert that only takes
// the row over when its lease has ended, an attempt that conflicts with a concurrent one is retried like any other
// held lock. the lease means a holder that died without unlocking doesn't block the others forever.
//
// the lock is taken by goose before it creates its version table or runs any migration, so concurrent callers,
// within a process or between processes sharing the database, apply the migrations one at a time and the later
// callers find nothing left to do.
//
// it is a session locker, not a lock.Locker, because it must run on the connection goose already holds. a Locker
// is handed the *sql.DB and needs a second connection, which deadlocks when connections are limited, e.g.) the
// duckproxy server hands out a few and every caller already holds one for its migration.
type duckdbLocker struct {
	owner string
	lease time.Duration
	wait  time.Duration
	poll  time.Duration
}

func newDuckdbLocker() *duckdbLocker {
	return &duckdbLocker{
		owner: uuid.Must(uuid.NewV7()).String(),
		lease: locklease,
		wait:  lockwait,
		poll:  lockpoll,
	}
}

var _ lock.SessionLocker = (*duckdbLocker)(nil)

// SessionLock blocks until the lock is held by t, the wait elapses, or ctx is done.
func (t *duckdbLocker) SessionLock(ctx context.Context, conn *sql.Conn) error {
	ctx, done := context.WithTimeout(ctx, t.wait)
	defer done()

	var last error
	for {
		acquired, err := t.try(ctx, conn)
		if acquired {
			return nil
		}

		// a conflict with another caller is expected and retried, remember the most recent for the timeout.
		if err != nil {
			last = err
		}

		select {
		case <-ctx.Done():
			return errorsx.Wrap(errorsx.Compact(ctx.Err(), last), "unable to acquire the migration lock")
		case <-time.After(t.poll):
		}
	}
}

// try to acquire the lock once. it is not an error for another caller to hold it, or for a concurrent attempt to
// conflict with this one, both are reported as not acquired.
func (t *duckdbLocker) try(ctx context.Context, conn *sql.Conn) (bool, error) {
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+locktable+` (
		id BIGINT PRIMARY KEY,
		owner VARCHAR NOT NULL,
		expires_at TIMESTAMPTZ NOT NULL
	)`); err != nil {
		return false, errorsx.Wrap(err, "unable to create the lock table")
	}

	now := time.Now().UTC()
	if _, err := conn.ExecContext(
		ctx,
		`INSERT INTO `+locktable+` (id, owner, expires_at) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET owner = excluded.owner, expires_at = excluded.expires_at
		WHERE `+locktable+`.expires_at < $4`,
		lockid, t.owner, now.Add(t.lease), now,
	); err != nil {
		return false, errorsx.Wrap(err, "unable to take the lock")
	}

	var holder string
	if err := conn.QueryRowContext(ctx, `SELECT owner FROM `+locktable+` WHERE id = $1`, lockid).Scan(&holder); err != nil {
		return false, errorsx.Wrap(err, "unable to read the lock")
	}

	return holder == t.owner, nil
}

// SessionUnlock releases the lock if t still holds it, it is retried while it conflicts with a concurrent attempt.
// a lock whose lease already ended and was taken over is not t's to release, that is not an error.
func (t *duckdbLocker) SessionUnlock(ctx context.Context, conn *sql.Conn) error {
	ctx, done := context.WithTimeout(ctx, t.wait)
	defer done()

	for {
		_, err := conn.ExecContext(ctx, `DELETE FROM `+locktable+` WHERE id = $1 AND owner = $2`, lockid, t.owner)
		if err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return errorsx.Wrap(errorsx.Compact(ctx.Err(), err), "unable to release the migration lock")
		case <-time.After(t.poll):
		}
	}
}
