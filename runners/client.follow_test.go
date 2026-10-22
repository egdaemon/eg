package runners_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/egdaemon/eg/runners"
	"github.com/golang-jwt/jwt/v4"
	"github.com/libp2p/go-libp2p"
	"github.com/stretchr/testify/require"
)

func TestFollowLogs(t *testing.T) {
	logtoken := func(t *testing.T, claims jwt.MapClaims) string {
		t.Helper()
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("secret"))
		require.NoError(t, err)
		return token
	}

	t.Run("streams the log from the runner", func(t *testing.T) {
		var (
			path, offset, auth string
			out                bytes.Buffer
		)

		client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer client.Close()

		runner := newRunner(t, client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path, offset, auth = r.URL.Path, r.URL.Query().Get("offset"), r.Header.Get("Authorization")
			_, _ = w.Write([]byte("hello world\n"))
		}))

		token := logtoken(t, jwt.MapClaims{"p2p": runner.ID().String()})

		ctx, done := context.WithTimeout(t.Context(), 10*time.Second)
		defer done()

		require.NoError(t, runners.FollowLogs(ctx, client, token, "w1", 6, &out))
		require.Equal(t, "hello world\n", out.String())
		require.Equal(t, "/c/logs/w1", path)
		require.Equal(t, "6", offset)
		require.Equal(t, token, auth)
	})

	t.Run("token without a runner", func(t *testing.T) {
		client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer client.Close()

		err = runners.FollowLogs(t.Context(), client, logtoken(t, jwt.MapClaims{}), "w1", 0, &bytes.Buffer{})
		require.ErrorContains(t, err, "log token missing runner")
	})

	t.Run("runner refuses the stream", func(t *testing.T) {
		var out bytes.Buffer

		client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer client.Close()

		runner := newRunner(t, client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))

		ctx, done := context.WithTimeout(t.Context(), 10*time.Second)
		defer done()

		err = runners.FollowLogs(ctx, client, logtoken(t, jwt.MapClaims{"p2p": runner.ID().String()}), "w1", 0, &out)
		require.Error(t, err)
		require.Empty(t, out.String())
	})

	t.Run("cancelling ends the stream", func(t *testing.T) {
		client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
		require.NoError(t, err)
		defer client.Close()

		release := make(chan struct{})
		defer close(release)

		runner := newRunner(t, client, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-release
		}))

		ctx, cancel := context.WithCancel(t.Context())
		result := make(chan error, 1)
		go func() {
			result <- runners.FollowLogs(ctx, client, logtoken(t, jwt.MapClaims{"p2p": runner.ID().String()}), "w1", 0, &bytes.Buffer{})
		}()

		time.Sleep(200 * time.Millisecond)
		cancel()

		select {
		case err := <-result:
			require.Error(t, err)
		case <-time.After(5 * time.Second):
			require.FailNow(t, "follow did not stop after cancel")
		}
	})
}
