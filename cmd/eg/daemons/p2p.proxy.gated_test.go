package daemons_test

import (
	"github.com/egdaemon/eg/cmd/migrations"
	"io"
	"net"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/stretchr/testify/require"
)

func TestP2PProxyGated(t *testing.T) {
	// the destination the daemon proxies streams to, it echoes everything back.
	httpl, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer httpl.Close()

	go func() {
		for {
			c, err := httpl.Accept()
			if err != nil {
				return
			}

			go func() {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}()
		}
	}()

	db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
	defer db.Close()
	runner, err := daemons.P2PProxyGated(t.Context(), []byte("p2p-proxy-gated-test"), httpl, daemons.NewLANGate(db, lan.DefaultTTL))
	require.NoError(t, err)
	defer runner.Close()

	t.Run("allowed peers are proxied and recorded", func(t *testing.T) {
		client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer client.Close()

		client.Peerstore().AddAddrs(runner.ID(), runner.Addrs(), peerstore.TempAddrTTL)
		s, err := client.NewStream(t.Context(), runner.ID(), "/egdaemon/proxy")
		require.NoError(t, err)
		defer s.Close()

		require.NoError(t, s.SetDeadline(time.Now().Add(5*time.Second)))
		_, err = s.Write([]byte("ping"))
		require.NoError(t, err)

		buf := make([]byte, 4)
		_, err = io.ReadFull(s, buf)
		require.NoError(t, err)
		require.Equal(t, "ping", string(buf))

		require.NoError(t, lan.PeerFindByID(t.Context(), db, client.ID().String()).Scan(&lan.Peer{}))
	})

	t.Run("disabled peers are reset", func(t *testing.T) {
		client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer client.Close()

		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(client.ID().String()))).Scan(&lan.Peer{}))
		require.NoError(t, lan.PeerUpdate(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(client.ID().String()), func(p *lan.Peer) {
			p.Disabled = true
		})).Scan(&lan.Peer{}))

		client.Peerstore().AddAddrs(runner.ID(), runner.Addrs(), peerstore.TempAddrTTL)
		s, err := client.NewStream(t.Context(), runner.ID(), "/egdaemon/proxy")
		require.NoError(t, err)
		defer s.Close()

		require.NoError(t, s.SetDeadline(time.Now().Add(5*time.Second)))
		_, _ = s.Write([]byte("ping"))

		_, err = io.ReadAll(s)
		require.Error(t, err, "the daemon resets the stream instead of proxying it")
	})

}
