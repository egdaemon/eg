package daemons_test

import (
	"github.com/egdaemon/eg/cmd/migrations"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

func TestLANGate(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519Key(nil)
	require.NoError(t, err)
	remote, err := peer.IDFromPrivateKey(priv)
	require.NoError(t, err)
	addr := multiaddr.StringCast("/ip4/192.168.1.20/tcp/4001")

	t.Run("records unknown peers and allows them", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()

		require.True(t, daemons.NewLANGate(db, lan.DefaultTTL).Allow(t.Context(), remote, addr))

		var p lan.Peer
		require.NoError(t, lan.PeerFindByID(t.Context(), db, remote.String()).Scan(&p))
		require.Equal(t, []string{"/ip4/192.168.1.20/tcp/4001"}, p.AddrList())
		require.NotEmpty(t, p.PublicKey)
		require.False(t, p.Disabled)
	})

	t.Run("refreshes known peers and keeps their settings", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(remote.String()))).Scan(&lan.Peer{}))
		sqltestx.Exec(t, db, `UPDATE lan_peers SET last_seen = now() - INTERVAL '10 days'`)

		require.True(t, daemons.NewLANGate(db, lan.DefaultTTL).Allow(t.Context(), remote, addr))

		var p lan.Peer
		require.NoError(t, lan.PeerFindByID(t.Context(), db, remote.String()).Scan(&p))
		require.WithinDuration(t, time.Now(), p.LastSeen, time.Minute)
		require.Equal(t, int64(8), p.Cores)
	})

	t.Run("refuses disabled peers", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		defer db.Close()
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(remote.String()))).Scan(&lan.Peer{}))
		require.NoError(t, lan.PeerUpdate(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(remote.String()), func(p *lan.Peer) {
			p.Disabled = true
		})).Scan(&lan.Peer{}))

		require.False(t, daemons.NewLANGate(db, lan.DefaultTTL).Allow(t.Context(), remote, addr))
	})

	t.Run("allows the stream when the database is unusable", func(t *testing.T) {
		db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
		require.NoError(t, db.Close())

		require.True(t, daemons.NewLANGate(db, lan.DefaultTTL).Allow(t.Context(), remote, addr))
	})
}
