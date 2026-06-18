package duckpgwire

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net"
	"sync"

	"github.com/jackc/pgx/v5/pgproto3"
)

// session holds the state of one client connection. Everything that
// touches sqlConn/dbTx runs on the single goroutine that owns the session
// -- the only exception is curCancel, which the cancel registry's
// goroutine may invoke concurrently in response to a CancelRequest, so
// it's guarded by mu.
type session struct {
	server  *Server
	conn    net.Conn
	backend *pgproto3.Backend

	// sqlConn is this session's dedicated database/sql connection to
	// duckproxy, held for the session's lifetime. dbTx is the active
	// transaction, or nil when idle -- see execer() for how statements
	// pick between the two.
	sqlConn *sql.Conn
	dbTx    *sql.Tx

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
		s.logger.Printf("duckpgwire: session ended: %v", err)
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

	s.sqlConn = sqlConn
	return s.serve(ctx)
}

func (s *session) serve(ctx context.Context) error {
	pid, secret := s.server.registry.register(s.cancelCurrent)
	s.pid, s.secret = pid, secret
	defer s.server.registry.deregister(pid)

	if err := s.sendHandshake(); err != nil {
		return err
	}

	defer s.cleanup()

	// duckproxy already enforces idle-in-transaction timeout server-side
	// on the real connection; duckpgwire doesn't need its own read
	// deadline / isTimeout handling to duplicate it.
	for {
		msg, err := s.backend.Receive()
		if err != nil {
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

// cleanup runs once, as the session ends. It rolls back any transaction
// the client left open so the underlying duckproxy connection comes
// back clean when this *sql.Conn is returned to the client-side pool.
func (s *session) cleanup() {
	if s.dbTx != nil {
		_ = s.dbTx.Rollback()
		s.dbTx = nil
	}
	s.statements = map[string]*preparedStatement{}
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

// statementContext derives a context for one statement's execution,
// bounded by the server's statement timeout (if any), and registers its
// cancel func so a CancelRequest can interrupt it. Cancelling it makes
// duckproxy's own client close its socket to the duckproxy server,
// which is how the in-flight statement actually gets interrupted
// server-side.
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
