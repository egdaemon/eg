package dbservice_test

import (
	"context"
	"database/sql"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/interp/dbproxy"
	"github.com/egdaemon/eg/interp/dbproxy/dbservice"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	_ "github.com/duckdb/duckdb-go/v2"
)

func fixture(t *testing.T, dir string) string {
	path := filepath.Join(dir, "cached.db")
	db, err := sql.Open("duckdb", path)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE "eg.metrics.coverage" (name VARCHAR); INSERT INTO "eg.metrics.coverage" VALUES ('a'), ('b')`)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	return path
}

func query(t *testing.T, socket string) int {
	db := sql.OpenDB(duckproxy.NewConnector(socket, func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}))
	defer db.Close()

	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM "eg.metrics.coverage"`).Scan(&n))
	return n
}

func TestServeAndRelease(t *testing.T) {
	ctx, done := context.WithTimeout(context.Background(), time.Minute)
	defer done()

	// unix socket paths are length limited, t.TempDir can exceed it.
	rundir, err := os.MkdirTemp("", "dbproxy")
	require.NoError(t, err)
	defer os.RemoveAll(rundir)

	path := fixture(t, t.TempDir())
	svc := dbservice.NewService(ctx, rundir)

	resp, err := svc.Serve(ctx, &dbproxy.ServeRequest{Path: path})
	require.NoError(t, err)
	require.Equal(t, dbservice.SocketName(path), resp.Socket)
	require.Equal(t, 2, query(t, filepath.Join(rundir, resp.Socket)))

	// serving the same path again shares the socket, it stays up until every serve is released.
	again, err := svc.Serve(ctx, &dbproxy.ServeRequest{Path: path})
	require.NoError(t, err)
	require.Equal(t, resp.Socket, again.Socket)

	_, err = svc.Release(ctx, &dbproxy.ReleaseRequest{Path: path})
	require.NoError(t, err)
	require.Equal(t, 2, query(t, filepath.Join(rundir, resp.Socket)))

	_, err = svc.Release(ctx, &dbproxy.ReleaseRequest{Path: path})
	require.NoError(t, err)
	require.NoFileExists(t, filepath.Join(rundir, resp.Socket))
}

func TestServeRejects(t *testing.T) {
	svc := dbservice.NewService(context.Background(), t.TempDir())

	_, err := svc.Serve(context.Background(), &dbproxy.ServeRequest{Path: "relative.db"})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = svc.Serve(context.Background(), &dbproxy.ServeRequest{Path: filepath.Join(t.TempDir(), "missing.db")})
	require.Equal(t, codes.NotFound, status.Code(err))
}
