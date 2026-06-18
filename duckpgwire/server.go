// Package duckpgwire is a Postgres-wire-protocol frontend for a duckproxy
// server: it translates Postgres frontend/backend messages into ordinary
// database/sql calls (ExecContext/QueryContext/BeginTx, plus
// duckproxy.Describe for type info) against a *sql.DB opened with the
// "duckproxy" driver, so that any off-the-shelf Postgres client can run
// SQL against a DuckDB database that duckproxy -- not this package --
// owns and pools connections to.
package duckpgwire

import (
	"context"
	"database/sql"
	"net"
	"time"
)

const (
	defaultMaxConnections   = 4
	defaultStatementTimeout = 30 * time.Second
)

// Server serves the Postgres wire protocol on behalf of a shared *sql.DB
// opened with the "duckproxy" driver (sql.Open("duckproxy", socketPath)).
// The zero value is not usable; construct with New.
type Server struct {
	db       *sql.DB
	logger   Logger
	registry *cancelRegistry

	maxConnections   int
	statementTimeout time.Duration
	acquireTimeout   time.Duration
}

// Option configures a Server constructed with New.
type Option func(*Server)

// WithMaxConnections caps the number of concurrent DuckDB connections handed
// out to client sessions, via db.SetMaxOpenConns. Additional clients block
// (queue) inside their Postgres handshake until a slot frees -- see
// Server.Serve for why the handshake, not the socket Accept, is the
// blocking point. Default: 4.
func WithMaxConnections(n int) Option {
	return func(s *Server) { s.maxConnections = n }
}

// WithStatementTimeout bounds how long any single statement execution may
// run before its context is cancelled (which DuckDB surfaces as an
// interrupt, not a connection kill). Default: 30s.
func WithStatementTimeout(d time.Duration) Option {
	return func(s *Server) { s.statementTimeout = d }
}

// WithAcquireTimeout bounds how long a session's startup handshake may wait
// for a free DuckDB connection slot before failing. Default: disabled (0 =
// wait indefinitely).
func WithAcquireTimeout(d time.Duration) Option {
	return func(s *Server) { s.acquireTimeout = d }
}

// WithLogger sets the Logger used to report errors that can't be surfaced
// to a client (e.g. a write failing after disconnect). Default: a no-op
// logger.
func WithLogger(l Logger) Option {
	return func(s *Server) { s.logger = l }
}

// New constructs a Server backed by db, which should be opened via
// sql.Open("duckproxy", socketPath) and dedicated to this Server -- New
// calls db.SetMaxOpenConns to enforce WithMaxConnections.
func New(db *sql.DB, opts ...Option) *Server {
	s := &Server{
		db:               db,
		logger:           noopLogger{},
		registry:         newCancelRegistry(),
		maxConnections:   defaultMaxConnections,
		statementTimeout: defaultStatementTimeout,
	}

	for _, opt := range opts {
		opt(s)
	}

	db.SetMaxOpenConns(s.maxConnections)

	return s
}

// Serve accepts connections from l and handles the Postgres wire protocol
// on each, until ctx is cancelled or l.Accept fails. It always returns a
// non-nil error.
func (s *Server) Serve(ctx context.Context, l net.Listener) error {
	go func() {
		<-ctx.Done()
		l.Close()
	}()

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}

		go s.handleConn(ctx, conn)
	}
}
