package duckproxy

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/jackc/pgx/v5/pgproto3"
)

// session holds the state of one client connection. Everything that
// touches dconn runs on the single goroutine that owns the session -- the
// only exception is curCancel, which the cancel registry's goroutine may
// invoke concurrently in response to a CancelRequest, so it's guarded by
// mu.
type session struct {
	server  *Server
	conn    net.Conn
	backend *pgproto3.Backend
	dconn   *duckdb.Conn

	pid    uint32
	secret uint32

	tx txState

	statements map[string]*preparedStatement
	portals    map[string]*portal

	mu        sync.Mutex
	curCancel context.CancelFunc
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	sess := &session{
		server:     s,
		conn:       conn,
		backend:    pgproto3.NewBackend(conn, conn),
		statements: make(map[string]*preparedStatement),
		portals:    make(map[string]*portal),
	}

	if err := sess.run(ctx); err != nil && !errors.Is(err, io.EOF) {
		s.logger.Printf("duckproxy: session ended: %v", err)
	}
}

func (s *session) run(ctx context.Context) error {
	proceed, err := s.negotiateStartup()
	if err != nil || !proceed {
		return err
	}

	acquireCtx := ctx
	if s.server.acquireTimeout > 0 {
		var done context.CancelFunc
		acquireCtx, done = context.WithTimeout(ctx, s.server.acquireTimeout)
		defer done()
	}

	sqlConn, err := s.server.db.Conn(acquireCtx)
	if err != nil {
		s.backend.Send(toErrorResponse(err))
		s.backend.Flush()
		return err
	}
	defer sqlConn.Close()

	// The entire rest of the session runs inside Raw: database/sql
	// documents that the driverConn it exposes "must not be used outside
	// of f", so the protocol loop -- not just a quick capture of dconn --
	// has to live in this closure.
	return sqlConn.Raw(func(driverConn any) error {
		s.dconn = driverConn.(*duckdb.Conn)
		return s.serve(ctx)
	})
}

func (s *session) serve(ctx context.Context) error {
	pid, secret := s.server.registry.register(s.cancelCurrent)
	s.pid, s.secret = pid, secret
	defer s.server.registry.deregister(pid)

	if err := s.sendHandshake(); err != nil {
		return err
	}

	defer s.cleanup()

	for {
		if s.tx == txInTransaction && s.server.idleInTransactionTimeout > 0 {
			s.conn.SetReadDeadline(time.Now().Add(s.server.idleInTransactionTimeout))
		} else {
			s.conn.SetReadDeadline(time.Time{})
		}

		msg, err := s.backend.Receive()
		if err != nil {
			if isTimeout(err) && s.tx == txInTransaction {
				s.killIdleInTransaction()
			}
			return err
		}

		switch m := msg.(type) {
		case *pgproto3.Query:
			s.handleQuery(ctx, m)
		case *pgproto3.Parse:
			s.handleParse(ctx, m)
		case *pgproto3.Bind:
			s.handleBind(m)
		case *pgproto3.Describe:
			s.handleDescribe(m)
		case *pgproto3.Execute:
			s.handleExecute(ctx, m)
		case *pgproto3.Sync:
			s.handleSync()
		case *pgproto3.Close:
			s.handleClose(m)
		case *pgproto3.Flush:
		case *pgproto3.Terminate:
			return nil
		}

		if err := s.backend.Flush(); err != nil {
			return err
		}
	}
}

// cleanup runs once, inside the Raw closure, as the session ends. It rolls
// back any transaction the client left open so the pooled connection comes
// back clean for whichever session is handed it next, and closes every
// statement still tracked for this session.
func (s *session) cleanup() {
	if s.tx != txIdle {
		_, _ = s.dconn.ExecContext(context.Background(), "ROLLBACK", nil)
	}
	for name, stmt := range s.statements {
		stmt.close()
		delete(s.statements, name)
	}
	s.portals = map[string]*portal{}
}

func (s *session) setCancel(cancel context.CancelFunc) {
	s.mu.Lock()
	s.curCancel = cancel
	s.mu.Unlock()
}

func (s *session) cancelCurrent() {
	s.mu.Lock()
	cancel := s.curCancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// killIdleInTransaction is called from the session's own goroutine after
// its read deadline expires while idle inside a transaction. It rolls
// back, then closes the connection directly -- a plain disconnect rather
// than a graceful FATAL ErrorResponse, since by this point the client has
// already gone quiet and there's no one left to read a reply.
func (s *session) killIdleInTransaction() {
	_, _ = s.dconn.ExecContext(context.Background(), "ROLLBACK", nil)
	s.tx = txIdle
	s.conn.Close()
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// statementContext derives a context for one statement's execution,
// bounded by the server's statement timeout (if any), and registers its
// cancel func so a CancelRequest can interrupt it.
func (s *session) statementContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.server.statementTimeout <= 0 {
		ctx, cancel := context.WithCancel(ctx)
		s.setCancel(cancel)
		return ctx, cancel
	}

	ctx, cancel := context.WithTimeout(ctx, s.server.statementTimeout)
	s.setCancel(cancel)
	return ctx, cancel
}
