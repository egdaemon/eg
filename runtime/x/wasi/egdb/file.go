package egdb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/dbproxy"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/wasinet/wasinet"
	"google.golang.org/grpc"
)

// File is a read only connection pool to a database file, see [OpenFile].
type File struct {
	*sql.DB
	path    string
	cc      *grpc.ClientConn
	release func(context.Context, string) error
}

// Close the connection pool and release the file, once every handle of the file has been closed it is no longer served.
func (t *File) Close() error {
	cerr := t.DB.Close()
	rerr := t.release(context.Background(), t.path)
	return errors.Join(cerr, t.cc.Close(), rerr)
}

// OpenFile opens the database file at path, e.g.) the file provided by [Latest], read only. the file is served by
// the runner separately from the analytics database, which is not touched. the connection is verified before
// returning; callers must Close the result.
func OpenFile(ctx context.Context, path string) (*File, error) {
	cc, err := egunsafe.DialControlSocket(ctx)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to connect to the runner")
	}

	client := dbproxy.NewDatabasesClient(cc)
	resp, err := client.Serve(ctx, &dbproxy.ServeRequest{Path: path})
	if err != nil {
		errorsx.Log(cc.Close())
		return nil, errorsx.Wrapf(err, "unable to serve %s", path)
	}

	f := &File{
		DB:   sql.OpenDB(duckproxy.NewConnector(egunsafe.RuntimeDirectory(resp.Socket), wasinet.DialContext)),
		path: path,
		cc:   cc,
		release: func(ctx context.Context, path string) error {
			_, err := client.Release(ctx, &dbproxy.ReleaseRequest{Path: path})
			return errorsx.Wrap(err, "unable to release the database")
		},
	}

	if err = f.PingContext(ctx); err != nil {
		errorsx.Log(f.Close())
		return nil, errorsx.Wrapf(err, "unable to connect to %s", path)
	}

	return f, nil
}
