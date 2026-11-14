// Package duckshare lets processes share a duckdb file, which only a single process may open.
package duckshare

import (
	"context"
	"crypto/md5"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"io"
	"maps"
	"net"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/duckproxy/duckgres"
	"github.com/egdaemon/duckproxy/duckproxy"
	"github.com/egdaemon/duckproxy/duckserve"
	"github.com/egdaemon/duckproxy/internal/errorsx"
	"github.com/gofrs/flock"
)

// Connector lets processes share a duckdb file, which only a single process may open. the process that
// connects first owns the file and serves it on a unix socket, the others connect through that socket. a file
// lock serializes connecting, so deciding who owns the file and setting up the socket never race, and it is
// only held while connecting. when the owner goes away the connections through it are discarded (see
// proxiedconn) and the next connect takes over, so the database recovers without the caller noticing.
type Connector struct {
	path string
	// sockdir holds the socket the owner serves the file on.
	sockdir string
	// lockpath is the file lock serializing connecting.
	lockpath string

	// lockwait is the longest a connection waits for another process to finish connecting.
	lockwait time.Duration
	// lockpoll is how often a held lock is retried.
	lockpoll time.Duration
	// dialwait is the longest it takes to decide whether the socket of the process holding the file is alive.
	dialwait time.Duration
	// maxsessions is how many connections of the others the owner serves at once.
	maxsessions int
	// pgsocket when set the owner also serves the file with the postgres wire protocol on this unix socket,
	// so clients that are not written in go, i.e. editor extensions, can use it.
	pgsocket string

	m     sync.Mutex
	owned *ownedfile
}

// ownedfile is the file held by this process and the socket it is served on.
type ownedfile struct {
	connector *duckdb.Connector
	db        *sql.DB // what the socket serves, it shares the connector.
	socket    string
	sessions  *sessions
	stop      context.CancelFunc
	done      chan struct{}
	refs      int

	// the postgres frontend, nil when not served.
	pgdb     *sql.DB
	pgsocket string
	pgdone   chan struct{}
}

// Option configures a Connector.
type Option func(*Connector)

// WithPostgresSocket also serves the file with the postgres wire protocol on the given unix socket while this
// process owns it, so clients that are not written in go, i.e. editor extensions, can use it.
func WithPostgresSocket(socket string) Option {
	return func(t *Connector) { t.pgsocket = socket }
}

// WithLockWait sets the longest a connection waits for another process to finish connecting, defaults to 10s.
func WithLockWait(d time.Duration) Option {
	return func(t *Connector) { t.lockwait = d }
}

// WithLockPath sets the file used to serialize connecting.
//
// Deprecated: exists so existing deployments keep the lock file they already use, new callers should use the
// default, the database path with the suffix ".duckproxy.lock".
func WithLockPath(path string) Option {
	return func(t *Connector) { t.lockpath = path }
}

// New shares the duckdb file at db between processes. sockdir is the directory, common to all the processes
// sharing the file, holding the socket the owner serves it on.
func New(sockdir, db string, opts ...Option) *Connector {
	t := &Connector{
		path:        db,
		sockdir:     sockdir,
		lockpath:    db + ".duckproxy.lock",
		lockwait:    10 * time.Second,
		lockpoll:    25 * time.Millisecond,
		dialwait:    time.Second,
		maxsessions: 128,
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

func (t *Connector) Driver() driver.Driver {
	return new(duckdb.Connector).Driver()
}

// socketpath is kept short, unix socket paths are length limited.
func (t *Connector) socketpath() string {
	sum := md5.Sum([]byte(t.path))
	return filepath.Join(t.sockdir, hex.EncodeToString(sum[:])[:16]+".socket")
}

func (t *Connector) Connect(ctx context.Context) (driver.Conn, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if t.owned != nil {
		return t.local(ctx)
	}

	lctx, done := context.WithTimeout(ctx, t.lockwait)
	defer done()

	lock := flock.New(t.lockpath)
	if locked, err := lock.TryLockContext(lctx, t.lockpoll); err != nil {
		return nil, errorsx.Wrap(err, "unable to wait for the database")
	} else if !locked {
		return nil, errorsx.New("unable to lock the database")
	}
	defer func() { errorsx.Log(lock.Unlock()) }()

	socket := t.socketpath()
	dctx, dcancel := context.WithTimeout(ctx, t.dialwait)
	defer dcancel()

	if probe, err := (&net.Dialer{}).DialContext(dctx, "unix", socket); err == nil {
		errorsx.Log(probe.Close())
		conn, err := duckproxy.NewConnector(socket, (&net.Dialer{}).DialContext).Connect(ctx)
		if gone(err) {
			return nil, driver.ErrBadConn
		} else if err != nil {
			return nil, err
		}

		return &proxiedconn{duckdbconn: conn.(duckdbconn)}, nil
	}

	if err := t.own(socket); err != nil {
		return nil, err
	}

	return t.local(ctx)
}

// own opens the file and serves it. the socket is listening before returning, so by the time the lock is
// released others find it.
func (t *Connector) own(socket string) error {
	connector, err := duckdb.NewConnector(t.path, nil)
	if err != nil {
		return errorsx.Wrap(err, "unable to open the database")
	}

	if err = os.MkdirAll(filepath.Dir(socket), 0700); err != nil {
		errorsx.Log(connector.Close())
		return errorsx.Wrap(err, "unable to create the socket directory")
	}

	// the dial failed so any socket is left over from a process that is gone.
	if err = os.Remove(socket); err != nil && !errors.Is(err, os.ErrNotExist) {
		errorsx.Log(connector.Close())
		return errorsx.Wrap(err, "unable to remove the stale socket")
	}

	l, err := net.Listen("unix", socket)
	if err != nil {
		errorsx.Log(connector.Close())
		return errorsx.Wrap(err, "unable to serve the database")
	}

	ctx, stop := context.WithCancel(context.Background())
	served := sql.OpenDB(connector)
	tracked := &sessions{Listener: l, conns: make(map[net.Conn]struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer l.Close()
		srv := duckserve.New(served, duckserve.WithMaxConnections(t.maxsessions), duckserve.WithAcquireTimeout(t.lockwait))
		if err := srv.Serve(ctx, tracked); err != nil && ctx.Err() == nil {
			errorsx.Log(errorsx.Wrap(err, "database proxy failed"))
		}
	}()

	t.owned = &ownedfile{connector: connector, db: served, socket: socket, sessions: tracked, stop: stop, done: done}

	if t.pgsocket != "" {
		// failing to serve the postgres frontend must not take the database down with it.
		if err = t.ownpg(ctx, socket); err != nil {
			errorsx.Log(errorsx.Wrap(err, "unable to serve the postgres frontend"))
		}
	}

	return nil
}

// ownpg serves the postgres wire protocol, it is a client of the native socket just like the others.
func (t *Connector) ownpg(ctx context.Context, socket string) (err error) {
	if err = os.MkdirAll(filepath.Dir(t.pgsocket), 0700); err != nil {
		return errorsx.Wrap(err, "unable to create the socket directory")
	}

	pgdb, err := sql.Open("duckproxy", socket)
	if err != nil {
		return errorsx.Wrap(err, "unable to open the database")
	}

	srv := duckgres.New(pgdb, duckgres.WithAcquireTimeout(t.lockwait))
	pgdone := make(chan struct{})
	go func() {
		defer close(pgdone)
		if err := duckgres.ListenUnix(ctx, t.pgsocket, srv); err != nil && ctx.Err() == nil {
			errorsx.Log(errorsx.Wrap(err, "postgres frontend failed"))
		}
	}()

	t.owned.pgdb = pgdb
	t.owned.pgsocket = t.pgsocket
	t.owned.pgdone = pgdone
	return nil
}

func (t *Connector) local(ctx context.Context) (driver.Conn, error) {
	conn, err := t.owned.connector.Connect(ctx)
	if err != nil {
		return nil, err
	}

	t.owned.refs++
	return &localconn{duckdbconn: conn.(duckdbconn), owner: t}, nil
}

func (t *Connector) released() {
	t.m.Lock()
	defer t.m.Unlock()
	t.owned.refs--
}

// Close stops serving the file and lets go of it, the connections of the others recover, see Connector.
func (t *Connector) Close() error {
	t.m.Lock()
	defer t.m.Unlock()

	if t.owned == nil {
		return nil
	}

	o := t.owned
	t.owned = nil

	// hang up before cancelling, otherwise a request racing the shutdown is answered with a cancelled
	// error instead of the connection going away.
	o.sessions.hangup()
	o.stop()
	<-o.done

	var pgerr error
	if o.pgdone != nil {
		<-o.pgdone
		_ = os.Remove(o.pgsocket)
		pgerr = o.pgdb.Close()
	}
	_ = os.Remove(o.socket)

	// closing the served database also closes the connector.
	return errors.Join(pgerr, o.db.Close())
}

// duckdbconn the interfaces database/sql uses of duckdb's connections, which the proxy's connections implement as well.
type duckdbconn interface {
	driver.Conn
	driver.ConnPrepareContext
	driver.ConnBeginTx
	driver.ExecerContext
	driver.QueryerContext
	driver.NamedValueChecker
}

// localconn is a connection to the file owned by this process.
type localconn struct {
	duckdbconn
	owner *Connector
	once  sync.Once
}

func (t *localconn) Close() error {
	defer t.once.Do(t.owner.released)
	return t.duckdbconn.Close()
}

// proxiedconn is a connection through the socket of the process owning the file. once that process is gone the
// connection is reported as bad so database/sql discards it and connects again, taking over the file.
type proxiedconn struct {
	duckdbconn
	broken bool
}

func (t *proxiedconn) IsValid() bool {
	return !t.broken
}

func (t *proxiedconn) check(err error) error {
	if gone(err) {
		t.broken = true
		return driver.ErrBadConn
	}

	return err
}

func gone(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNREFUSED)
}

func (t *proxiedconn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	s, err := t.duckdbconn.PrepareContext(ctx, query)
	return s, t.check(err)
}

func (t *proxiedconn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	tx, err := t.duckdbconn.BeginTx(ctx, opts)
	return tx, t.check(err)
}

func (t *proxiedconn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	r, err := t.duckdbconn.ExecContext(ctx, query, args)
	return r, t.check(err)
}

func (t *proxiedconn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	r, err := t.duckdbconn.QueryContext(ctx, query, args)
	return r, t.check(err)
}

// sessions hangs up on the connections of the others when the owner stops, serving stops accepting but leaves
// them open and they'd answer with errors instead of letting their connections know to recover.
type sessions struct {
	net.Listener

	m      sync.Mutex
	conns  map[net.Conn]struct{}
	closed bool
}

func (t *sessions) Accept() (net.Conn, error) {
	c, err := t.Listener.Accept()
	if err != nil {
		return nil, err
	}

	t.m.Lock()
	defer t.m.Unlock()

	if t.closed {
		_ = c.Close()
		return nil, net.ErrClosed
	}

	tc := &sessionconn{Conn: c, owner: t}
	t.conns[tc] = struct{}{}

	return tc, nil
}

// hangup closes the connections, returning once they are closed.
func (t *sessions) hangup() {
	t.m.Lock()
	t.closed = true
	conns := slices.Collect(maps.Keys(t.conns))
	t.m.Unlock()

	for _, c := range conns {
		_ = c.Close()
	}
}

type sessionconn struct {
	net.Conn
	owner *sessions
}

func (t *sessionconn) Close() error {
	t.owner.m.Lock()
	delete(t.owner.conns, t)
	t.owner.m.Unlock()

	return t.Conn.Close()
}
