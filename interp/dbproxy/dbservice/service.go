// Package dbservice is the host side of dbproxy: it serves database files to modules, each over its own duckproxy socket,
// so modules can query them without opening the file themselves or touching the analytics database.
// it is not available to modules, which use the generated client in dbproxy.
package dbservice

import (
	"context"
	"database/sql"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/interp/dbproxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// SocketName is the name of the socket serving the database file at path (as the module sees it).
func SocketName(path string) string {
	return "db." + md5x.String(path) + ".socket"
}

// NewService serves databases on sockets within runtimeDir until ctx is cancelled.
func NewService(ctx context.Context, runtimeDir string) *Service {
	return &Service{
		ctx:        ctx,
		runtimeDir: runtimeDir,
		served:     make(map[string]*served),
	}
}

type served struct {
	refs int
	stop context.CancelFunc
	done chan struct{}
}

type Service struct {
	dbproxy.UnimplementedDatabasesServer
	ctx        context.Context
	runtimeDir string
	mu         sync.Mutex
	served     map[string]*served
}

func (t *Service) Bind(host grpc.ServiceRegistrar) {
	dbproxy.RegisterDatabasesServer(host, t)
}

// Serve implements DatabasesServer.
func (t *Service) Serve(ctx context.Context, req *dbproxy.ServeRequest) (*dbproxy.ServeResponse, error) {
	if !filepath.IsAbs(req.Path) {
		return nil, status.Error(codes.InvalidArgument, "path must be absolute")
	}

	if _, err := os.Stat(req.Path); os.IsNotExist(err) {
		return nil, status.Error(codes.NotFound, "database not found")
	} else if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	socket := SocketName(req.Path)

	t.mu.Lock()
	defer t.mu.Unlock()

	if s, ok := t.served[req.Path]; ok {
		s.refs++
		return &dbproxy.ServeResponse{Socket: socket}, nil
	}

	s, err := t.start(req.Path, filepath.Join(t.runtimeDir, socket))
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	t.served[req.Path] = s

	return &dbproxy.ServeResponse{Socket: socket}, nil
}

// start serving the database, the socket accepts connections once start returns.
func (t *Service) start(path string, socket string) (_ *served, err error) {
	db, err := sql.Open("duckdb", path+"?access_mode=read_only")
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to open database")
	}

	if err = db.PingContext(t.ctx); err != nil {
		errorsx.Log(db.Close())
		return nil, errorsx.Wrap(err, "unable to open database")
	}

	if err = os.Remove(socket); err != nil && !os.IsNotExist(err) {
		errorsx.Log(db.Close())
		return nil, errorsx.Wrap(err, "unable to remove stale socket")
	}

	l, err := net.Listen("unix", socket)
	if err != nil {
		errorsx.Log(db.Close())
		return nil, errorsx.Wrap(err, "unable to listen")
	}

	sctx, stop := context.WithCancel(t.ctx)
	s := &served{refs: 1, stop: stop, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		defer db.Close()
		defer os.Remove(socket)
		_ = duckproxyserver.New(db).Serve(sctx, l)
	}()

	return s, nil
}

// Release implements DatabasesServer.
func (t *Service) Release(ctx context.Context, req *dbproxy.ReleaseRequest) (*dbproxy.ReleaseResponse, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.served[req.Path]
	if !ok {
		return &dbproxy.ReleaseResponse{}, nil
	}

	if s.refs--; s.refs > 0 {
		return &dbproxy.ReleaseResponse{}, nil
	}

	// wait for the server to finish with the socket before it can be served again.
	s.stop()
	<-s.done
	delete(t.served, req.Path)

	return &dbproxy.ReleaseResponse{}, nil
}
