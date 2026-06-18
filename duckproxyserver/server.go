// Package server fronts a single shared in-process DuckDB database with
// duckproxy's native Go protocol over a Unix domain socket, so that
// multiple separate OS processes can connect -- via duckproxy's
// database/sql/driver.Driver client -- and run SQL against one DuckDB
// file, the way pgpool/pgbouncer let many Postgres clients share one
// backend. Unlike a Postgres-wire-protocol proxy, both ends speak DuckDB's
// own native types directly; only Go code using duckproxy's client can
// connect. This package needs the real duckdb-go driver (and its cgo
// bindings) to actually execute SQL, unlike duckproxy itself, which stays
// cgo-free so it can be imported from the wasm guest.
package duckproxyserver

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

// Server serves duckproxy's native protocol on behalf of a shared
// *sql.DB backed by the duckdb driver. The zero value is not usable;
// construct with New.
type Server struct {
	db     *sql.DB
	logger Logger

	maxConnections           int
	idleInTransactionTimeout time.Duration
	statementTimeout         time.Duration
	acquireTimeout           time.Duration
}

// Option configures a Server constructed with New.
type Option func(*Server)

// WithMaxConnections caps the number of concurrent DuckDB connections handed
// out to client sessions, via db.SetMaxOpenConns. Additional clients block
// (queue) waiting to acquire a connection -- see Server.serve for why that
// happens before any frame is exchanged, not at the socket Accept. Default: 4.
func WithMaxConnections(n int) Option {
	return func(s *Server) { s.maxConnections = n }
}

// WithIdleInTransactionTimeout closes a session and rolls back its open
// transaction if it sits idle inside Begin/Commit for longer than d. This
// guards against one stalled client monopolizing DuckDB's single active
// write-transaction slot. Default: disabled (0).
func WithIdleInTransactionTimeout(d time.Duration) Option {
	return func(s *Server) { s.idleInTransactionTimeout = d }
}

// WithStatementTimeout bounds how long any single statement execution may
// run before its context is cancelled (which DuckDB surfaces as an
// interrupt, not a connection kill). Default: 30s.
func WithStatementTimeout(d time.Duration) Option {
	return func(s *Server) { s.statementTimeout = d }
}

// WithAcquireTimeout bounds how long a new session may wait for a free
// DuckDB connection slot before failing. Default: disabled (0 = wait
// indefinitely).
func WithAcquireTimeout(d time.Duration) Option {
	return func(s *Server) { s.acquireTimeout = d }
}

// WithLogger sets the Logger used to report errors that can't be surfaced
// to a client (e.g. a write failing after disconnect). Default: a no-op
// logger.
func WithLogger(l Logger) Option {
	return func(s *Server) { s.logger = l }
}

// New constructs a Server backed by db. db should be dedicated to this
// Server -- New calls db.SetMaxOpenConns to enforce WithMaxConnections.
func New(db *sql.DB, opts ...Option) *Server {
	s := &Server{
		db:               db,
		logger:           noopLogger{},
		maxConnections:   defaultMaxConnections,
		statementTimeout: defaultStatementTimeout,
	}

	for _, opt := range opts {
		opt(s)
	}

	db.SetMaxOpenConns(s.maxConnections)

	return s
}

// Serve accepts connections from l and handles duckproxy's protocol on
// each, until ctx is cancelled or l.Accept fails. It always returns a
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
