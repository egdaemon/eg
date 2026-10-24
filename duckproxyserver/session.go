package duckproxyserver

import (
	"context"
	"database/sql/driver"
	"errors"
	"io"
	"net"
	"time"

	duckdb "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/duckproxy"
)

// session holds the state of one client connection, and is only ever
// touched by the single goroutine that owns it -- there is no concurrent
// cancellation mechanism in this protocol (see Server doc comment); a
// caller that wants to interrupt an in-flight statement closes its
// connection instead.
type session struct {
	server *Server
	conn   net.Conn
	dconn  *duckdb.Conn

	// tx is the active transaction, or nil if none is open. Its presence
	// is what idle-in-transaction timeout and the rollback-on-cleanup
	// safety net key off of.
	tx driver.Tx
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	acquireCtx := ctx
	if s.acquireTimeout > 0 {
		var done context.CancelFunc
		acquireCtx, done = context.WithTimeout(ctx, s.acquireTimeout)
		defer done()
	}

	sqlConn, err := s.db.Conn(acquireCtx)
	if err != nil {
		duckproxy.WriteFrame(conn, &duckproxy.ServerFrame{Body: &duckproxy.ServerFrame_Error{Error: &duckproxy.ErrorResponse{Message: err.Error()}}})
		return
	}
	defer sqlConn.Close()

	// The entire session runs inside Raw: database/sql documents that the
	// driverConn it exposes "must not be used outside of f", so the
	// dispatch loop -- not just a quick capture of dconn -- has to live in
	// this closure.
	err = sqlConn.Raw(func(driverConn any) error {
		sess := &session{server: s, conn: conn, dconn: driverConn.(*duckdb.Conn)}
		return sess.serve(ctx)
	})
	if err != nil && !errors.Is(err, io.EOF) {
		s.logger.Printf("duckproxy: session ended: %v", err)
	}
}

func (s *session) serve(ctx context.Context) error {
	defer s.cleanup()

	for {
		if s.tx != nil && s.server.idleInTransactionTimeout > 0 {
			s.conn.SetReadDeadline(time.Now().Add(s.server.idleInTransactionTimeout))
		} else {
			s.conn.SetReadDeadline(time.Time{})
		}

		var frame duckproxy.ClientFrame
		if err := duckproxy.ReadFrame(s.conn, &frame); err != nil {
			if isTimeout(err) && s.tx != nil {
				s.killIdleInTransaction()
			}
			return err
		}

		var err error
		switch body := frame.GetBody().(type) {
		case *duckproxy.ClientFrame_Exec:
			err = s.handleExec(ctx, body.Exec)
		case *duckproxy.ClientFrame_Query:
			err = s.handleQuery(ctx, body.Query)
		case *duckproxy.ClientFrame_Begin:
			err = s.handleBegin(ctx)
		case *duckproxy.ClientFrame_Commit:
			err = s.handleCommit()
		case *duckproxy.ClientFrame_Rollback:
			err = s.handleRollback()
		case *duckproxy.ClientFrame_Describe:
			err = s.handleDescribe(ctx, body.Describe)
		default:
			err = s.sendError(errors.New("duckproxy: empty or unknown client frame"))
		}

		if err != nil {
			return err
		}
	}
}

// cleanup runs once, inside the Raw closure, as the session ends. It rolls
// back any transaction the client left open so the pooled connection comes
// back clean for whichever session is handed it next.
func (s *session) cleanup() {
	if s.tx != nil {
		_ = s.tx.Rollback()
		s.tx = nil
	}
}

// killIdleInTransaction is called from the session's own goroutine after
// its read deadline expires while idle inside a transaction. It rolls
// back, then closes the connection directly -- a plain disconnect, since
// by this point the client has gone quiet and there's no one left to read
// a reply.
func (s *session) killIdleInTransaction() {
	if s.tx != nil {
		_ = s.tx.Rollback()
		s.tx = nil
	}
	s.conn.Close()
}

func (s *session) sendError(err error) error {
	return duckproxy.WriteFrame(s.conn, &duckproxy.ServerFrame{Body: &duckproxy.ServerFrame_Error{Error: &duckproxy.ErrorResponse{Message: err.Error()}}})
}

// statementContext derives a context for one statement's execution,
// bounded by the server's statement timeout, if any.
func (s *session) statementContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.server.statementTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, s.server.statementTimeout)
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func toNamedValues(params []*duckproxy.Param) ([]driver.NamedValue, error) {
	args := make([]driver.NamedValue, len(params))
	for i, p := range params {
		v, err := fromProtoValue(p.GetValue())
		if err != nil {
			return nil, err
		}
		args[i] = driver.NamedValue{Ordinal: int(p.GetOrdinal()), Name: p.GetName(), Value: v}
	}
	return args, nil
}
