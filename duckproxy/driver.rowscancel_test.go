package duckproxy_test

import (
	"context"
	"database/sql"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/duckproxy"
	"github.com/stretchr/testify/require"
)

// stallingServer listens on a unix socket and answers every Query with a Columns frame
// and then goes silent, like a server stuck mid-statement. it holds the connection
// open until the test ends.
func stallingServer(t *testing.T) (socketPath string) {
	t.Helper()

	socketPath = filepath.Join(t.TempDir(), "stall.sock")
	l, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })

	stop := make(chan struct{})
	t.Cleanup(func() { close(stop) })

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}

			go func() {
				defer c.Close()

				var frame duckproxy.ClientFrame
				if err := duckproxy.ReadFrame(c, &frame); err != nil {
					return
				}

				if err := duckproxy.WriteFrame(c, &duckproxy.ServerFrame{Body: &duckproxy.ServerFrame_Columns{Columns: &duckproxy.ColumnsResponse{Names: []string{"id"}}}}); err != nil {
					return
				}

				<-stop
			}()
		}
	}()

	return socketPath
}

// a context that expires while the server is streaming rows must unblock the client.
func TestDriverQueryContextCancelledWhileStreaming(t *testing.T) {
	db, err := sql.Open("duckproxy", stallingServer(t))
	require.NoError(t, err)

	ctx, done := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer done()

	rows, err := db.QueryContext(ctx, "SELECT 1")
	require.NoError(t, err)

	finished := make(chan error, 1)
	go func() {
		defer rows.Close()
		for rows.Next() {
		}
		finished <- rows.Err()
	}()

	select {
	case err := <-finished:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("rows iteration did not return after the context expired")
	}
}

// the server never answers the query, the context must unblock QueryContext itself.
func TestDriverQueryContextCancelledBeforeColumns(t *testing.T) {
	socketPath := filepath.Join(t.TempDir(), "silent.sock")
	l, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	t.Cleanup(func() { l.Close() })

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { c.Close() })
		}
	}()

	db, err := sql.Open("duckproxy", socketPath)
	require.NoError(t, err)

	ctx, done := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer done()

	finished := make(chan error, 1)
	go func() {
		rows, err := db.QueryContext(ctx, "SELECT 1")
		if err == nil {
			rows.Close()
		}
		finished <- err
	}()

	select {
	case err := <-finished:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("QueryContext did not return after the context expired")
	}
}
