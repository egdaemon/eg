package netx_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/netx"
	"github.com/stretchr/testify/require"
)

func unixlistener(t *testing.T, name string) net.Listener {
	t.Helper()
	l, err := net.Listen("unix", filepath.Join(t.TempDir(), name))
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	return l
}

// proxied accepts a single connection on a frontend listener and proxies it to
// the backend; returns the frontend address and the result of the proxy.
func proxied(t *testing.T, ctx context.Context, backend net.Addr) (net.Addr, chan error) {
	t.Helper()
	frontend := unixlistener(t, "frontend.socket")
	result := make(chan error, 1)

	go func() {
		src, err := frontend.Accept()
		if err != nil {
			result <- err
			return
		}
		defer src.Close()

		dst, err := net.Dial(backend.Network(), backend.String())
		if err != nil {
			result <- err
			return
		}
		defer dst.Close()

		result <- netx.Proxy(ctx, src, dst)
	}()

	return frontend.Addr(), result
}

func awaitProxy(t *testing.T, result chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		require.FailNow(t, "proxy did not complete")
		return nil
	}
}

func TestProxy(t *testing.T) {
	t.Run("completes once the client closes a keep alive http connection", func(t *testing.T) {
		backend := unixlistener(t, "backend.socket")
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = io.WriteString(w, "hello")
		})}
		go func() { _ = srv.Serve(backend) }()
		t.Cleanup(func() { _ = srv.Close() })

		addr, result := proxied(t, t.Context(), backend.Addr())
		transport := &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, addr.Network(), addr.String())
			},
		}
		c := &http.Client{Transport: transport}

		resp, err := c.Get("http://proxied/")
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, "hello", string(body))

		transport.CloseIdleConnections()
		require.NoError(t, awaitProxy(t, result))
	})

	t.Run("context cancellation tears down idle connections", func(t *testing.T) {
		backend := unixlistener(t, "backend.socket")
		go func() {
			conn, err := backend.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			_, _ = io.Copy(io.Discard, conn)
		}()

		ctx, cancel := context.WithCancel(t.Context())
		addr, result := proxied(t, ctx, backend.Addr())
		conn, err := net.Dial(addr.Network(), addr.String())
		require.NoError(t, err)
		defer conn.Close()

		time.Sleep(50 * time.Millisecond)
		cancel()
		awaitProxy(t, result)
	})
}
