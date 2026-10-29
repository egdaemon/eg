package migrations

import (
	"context"
	"database/sql"
	"database/sql/driver"
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
	"github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/gofrs/flock"
)

// sharedconnector lets processes share a duckdb file, which only a single process may open. the process that
// connects first owns the file and serves it on a unix socket, the others connect through that socket. a file
// lock serializes connecting, so deciding who owns the file and setting up the socket never race, and it is
// only held while connecting. when the owner goes away the connections through it are discarded (see
// proxiedconn) and the next connect takes over, so the database recovers without the caller noticing.
type sharedconnector struct {
	path string

	// lockwait is the longest a connection waits for another process to finish connecting.
	lockwait time.Duration
	// lockpoll is how often a held lock is retried.
	lockpoll time.Duration
	// dialwait is the longest it takes to decide whether the socket of the process holding the file is alive.
	dialwait time.Duration
	// maxsessions is how many connections of the others the owner serves at once.
	maxsessions int

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
}

func newsharedconnector(path string) *sharedconnector {
	return &sharedconnector{
		path:        path,
		lockwait:    10 * time.Second,
		lockpoll:    25 * time.Millisecond,
		dialwait:    time.Second,
		maxsessions: 128,
	}
}

func (t *sharedconnector) Driver() driver.Driver {
	return new(duckdb.Connector).Driver()
}

// socketpath is kept short, unix socket paths are length limited.
func (t *sharedconnector) socketpath() string {
	return userx.DefaultRuntimeDirectory(md5x.FormatNHex(16, md5x.Digest(t.path)) + ".socket")
}

func (t *sharedconnector) Connect(ctx context.Context) (driver.Conn, error) {
	t.m.Lock()
	defer t.m.Unlock()

	if t.owned != nil {
		return t.local(ctx)
	}

	lctx, done := context.WithTimeout(ctx, t.lockwait)
	defer done()

	lock := flock.New(t.path + ".lock")
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
func (t *sharedconnector) own(socket string) error {
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
		srv := duckproxyserver.New(served, duckproxyserver.WithMaxConnections(t.maxsessions), duckproxyserver.WithAcquireTimeout(t.lockwait))
		if err := srv.Serve(ctx, tracked); err != nil && ctx.Err() == nil {
			errorsx.Log(errorsx.Wrap(err, "database proxy failed"))
		}
	}()

	t.owned = &ownedfile{connector: connector, db: served, socket: socket, sessions: tracked, stop: stop, done: done}
	return nil
}

func (t *sharedconnector) local(ctx context.Context) (driver.Conn, error) {
	conn, err := t.owned.connector.Connect(ctx)
	if err != nil {
		return nil, err
	}

	t.owned.refs++
	return &localconn{duckdbconn: conn.(duckdbconn), owner: t}, nil
}

func (t *sharedconnector) released() {
	t.m.Lock()
	defer t.m.Unlock()
	t.owned.refs--
}

// Close stops serving the file and lets go of it, the connections of the others recover, see sharedconnector.
func (t *sharedconnector) Close() error {
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
	_ = os.Remove(o.socket)

	// closing the served database also closes the connector.
	return errors.Join(o.db.Close())
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
	owner *sharedconnector
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
