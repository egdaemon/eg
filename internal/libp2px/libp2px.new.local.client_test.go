package libp2px_test

import (
	"io"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/stretchr/testify/require"
)

func TestNewLocalClient(t *testing.T) {
	runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
	require.NoError(t, err)
	defer runner.Close()

	runner.SetStreamHandler("/test/echo", func(s network.Stream) {
		defer s.Close()
		_, _ = io.Copy(s, s)
	})

	client, err := libp2px.NewLocalClient()
	require.NoError(t, err)
	defer client.Close()

	t.Run("reaches a peer using only the addresses it was given", func(t *testing.T) {
		client.Peerstore().AddAddrs(runner.ID(), runner.Addrs(), peerstore.TempAddrTTL)

		s, err := client.NewStream(t.Context(), runner.ID(), "/test/echo")
		require.NoError(t, err)
		defer s.Close()

		require.NoError(t, s.SetDeadline(time.Now().Add(5*time.Second)))
		_, err = s.Write([]byte("ping"))
		require.NoError(t, err)

		buf := make([]byte, 4)
		_, err = io.ReadFull(s, buf)
		require.NoError(t, err)
		require.Equal(t, "ping", string(buf))
	})

	t.Run("cannot find peers it wasn't told about", func(t *testing.T) {
		stranger, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer stranger.Close()

		_, err = client.NewStream(t.Context(), stranger.ID(), "/test/echo")
		require.Error(t, err)
	})
}
