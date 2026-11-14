// Package computesess registers 'eg compute serve' workloads in the cli's database so tools, i.e. an editor
// extension, can discover how to connect to the running ones and restart the ones that stopped.
package computesess

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"syscall"

	"github.com/egdaemon/eg/internal/errorsx"
)

// HistoryLimit is how many stopped sessions are kept.
const HistoryLimit = 50

// Register the session as running, first marking the sessions whose process is gone as stopped, i.e. killed before they could stop.
// registering an id of a stopped session restarts it.
func Register(ctx context.Context, db *sql.DB, s Session) (err error) {
	if err = reap(ctx, db); err != nil {
		return err
	}

	if s.Pid == 0 {
		s.Pid = int64(os.Getpid())
	}

	// ids are picked by clients, never take over the id of a session that is still running in another process.
	if existing, lerr := Lookup(ctx, db, s.ID); lerr == nil {
		if existing.Running() && existing.Pid != s.Pid {
			return errorsx.Errorf("serve session %s is already running as process %d", s.ID, existing.Pid)
		}
	} else if !errors.Is(lerr, sql.ErrNoRows) {
		return lerr
	}

	if err = SessionUpsert(ctx, db, s).Scan(&s); err != nil {
		return errorsx.Wrap(err, "unable to register serve session")
	}

	return prune(ctx, db)
}

// Stop marks the session as stopped. the session is kept as history.
func Stop(ctx context.Context, db *sql.DB, id string) error {
	var stopped Session

	// no rows means the session is not running, which is the state asked for.
	if err := SessionStop(ctx, db, id).Scan(&stopped); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errorsx.Wrap(err, "unable to stop serve session")
	}

	return nil
}

// Lookup a session by id, running or stopped. returns sql.ErrNoRows when there is none.
func Lookup(ctx context.Context, db *sql.DB, id string) (s Session, err error) {
	if err = reap(ctx, db); err != nil {
		return s, err
	}

	if err = SessionFindByID(ctx, db, id).Scan(&s); errors.Is(err, sql.ErrNoRows) {
		return s, err
	}

	return s, errorsx.Wrap(err, "unable to lookup serve session")
}

// List the sessions, the running ones first and then the most recently started.
func List(ctx context.Context, db *sql.DB) (sessions []Session, err error) {
	if err = reap(ctx, db); err != nil {
		return nil, err
	}

	if sessions, err = collect(SessionLookupAll(ctx, db)); err != nil {
		return nil, errorsx.Wrap(err, "unable to list serve sessions")
	}

	return sessions, nil
}

// reap marks the running sessions whose process is gone as stopped.
func reap(ctx context.Context, db *sql.DB) (err error) {
	running, err := collect(SessionLookupRunning(ctx, db))
	if err != nil {
		return errorsx.Wrap(err, "unable to inspect serve sessions")
	}

	for _, s := range running {
		// signal 0 only checks for existence. EPERM means the process exists.
		if perr := syscall.Kill(int(s.Pid), 0); perr != syscall.ESRCH {
			continue
		}

		if err = Stop(ctx, db, s.ID); err != nil {
			return err
		}
	}

	return nil
}

// prune forgets the stopped sessions beyond the history limit, oldest first.
func prune(ctx context.Context, db *sql.DB) error {
	_, err := collect(SessionPrune(ctx, db, HistoryLimit))
	return errorsx.Wrap(err, "unable to prune serve sessions")
}

// collect reads every session the scanner returns and closes it.
func collect(scanner SessionScanner) (sessions []Session, err error) {
	defer func() {
		err = errors.Join(err, scanner.Close())
	}()

	for scanner.Next() {
		var s Session
		if err = scanner.Scan(&s); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}

	return sessions, scanner.Err()
}
