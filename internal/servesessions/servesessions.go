// Package servesessions registers 'eg compute serve' workloads in the cli's database so tools, i.e. an editor
// extension, can discover how to connect to the running ones and restart the ones that stopped.
package servesessions

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"syscall"
	"time"

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

	if s.PID == 0 {
		s.PID = int64(os.Getpid())
	}

	// ids are picked by clients, never take over the id of a session that is still running in another process.
	if existing, lerr := Lookup(ctx, db, s.ID); lerr == nil {
		if existing.Running() && existing.PID != s.PID {
			return errorsx.Errorf("serve session %s is already running as process %d", s.ID, existing.PID)
		}
	} else if !errors.Is(lerr, sql.ErrNoRows) {
		return lerr
	}

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO serve_sessions (id, module, directory, pid, ssh_requested, ssh_port, ssh_user, ssh_key_path, started_at, stopped_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, now(), NULL)
		ON CONFLICT (id) DO UPDATE SET
			module = excluded.module, directory = excluded.directory, pid = excluded.pid, ssh_requested = excluded.ssh_requested,
			ssh_port = excluded.ssh_port, ssh_user = excluded.ssh_user, ssh_key_path = excluded.ssh_key_path,
			-- the port changing within a run, i.e. another module's container, is not a restart.
			started_at = CASE WHEN serve_sessions.stopped_at IS NULL THEN serve_sessions.started_at ELSE now() END,
			stopped_at = NULL`,
		s.ID, s.Module, s.Directory, s.PID, s.SSHRequested, s.SSHPort, s.SSHUser, s.SSHKeyPath,
	)
	if err != nil {
		return errorsx.Wrap(err, "unable to register serve session")
	}

	return prune(ctx, db)
}

// Stop marks the session as stopped. the session is kept as history.
func Stop(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `UPDATE serve_sessions SET stopped_at = now(), ssh_port = 0 WHERE id = ? AND stopped_at IS NULL`, id)
	return errorsx.Wrap(err, "unable to stop serve session")
}

// Lookup a session by id, running or stopped. returns sql.ErrNoRows when there is none.
func Lookup(ctx context.Context, db *sql.DB, id string) (s Session, err error) {
	if err = reap(ctx, db); err != nil {
		return s, err
	}

	s, err = scan(db.QueryRowContext(ctx, `SELECT `+columns+` FROM serve_sessions WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return s, err
	}

	return s, errorsx.Wrap(err, "unable to lookup serve session")
}

// List the sessions, the running ones first and then the most recently started.
func List(ctx context.Context, db *sql.DB) (sessions []Session, err error) {
	if err = reap(ctx, db); err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, `SELECT `+columns+` FROM serve_sessions ORDER BY stopped_at IS NOT NULL, started_at DESC`)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to list serve sessions")
	}
	defer rows.Close()

	for rows.Next() {
		var s Session
		if s, err = scan(rows); err != nil {
			return nil, errorsx.Wrap(err, "unable to scan serve session")
		}
		sessions = append(sessions, s)
	}

	return sessions, errorsx.Wrap(rows.Err(), "unable to list serve sessions")
}

// reap marks the running sessions whose process is gone as stopped.
func reap(ctx context.Context, db *sql.DB) (err error) {
	rows, err := db.QueryContext(ctx, `SELECT id, pid FROM serve_sessions WHERE stopped_at IS NULL`)
	if err != nil {
		return errorsx.Wrap(err, "unable to inspect serve sessions")
	}

	var dead []string
	for rows.Next() {
		var (
			id  string
			pid int64
		)
		if err = rows.Scan(&id, &pid); err != nil {
			errorsx.Log(rows.Close())
			return errorsx.Wrap(err, "unable to scan serve session")
		}

		// signal 0 only checks for existence. EPERM means the process exists.
		if perr := syscall.Kill(int(pid), 0); perr == syscall.ESRCH {
			dead = append(dead, id)
		}
	}
	err = rows.Err()
	errorsx.Log(rows.Close())
	if err != nil {
		return errorsx.Wrap(err, "unable to inspect serve sessions")
	}

	for _, id := range dead {
		if err = Stop(ctx, db, id); err != nil {
			return err
		}
	}

	return nil
}

// prune forgets the stopped sessions beyond the history limit, oldest first.
func prune(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(
		ctx,
		`DELETE FROM serve_sessions WHERE stopped_at IS NOT NULL AND id NOT IN (
			SELECT id FROM serve_sessions WHERE stopped_at IS NOT NULL ORDER BY stopped_at DESC LIMIT ?
		)`,
		HistoryLimit,
	)
	return errorsx.Wrap(err, "unable to prune serve sessions")
}
