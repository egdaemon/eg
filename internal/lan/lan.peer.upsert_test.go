package lan_test

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/stretchr/testify/require"
)

func TestPeerUpsert(t *testing.T) {
	db := sqltestx.DuckDB(t)
	require.NoError(t, lan.Migrate(t.Context(), db))

	var got lan.Peer
	require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"))).Scan(&got))
	require.Equal(t, "a", got.PeerID)
	require.False(t, got.FirstSeen.IsZero())
	require.False(t, got.LastSeen.IsZero())
	require.False(t, got.Disabled)
	require.Zero(t, got.ExpiresAfter)

	t.Run("resources round trip", func(t *testing.T) {
		want := langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"))
		require.Equal(t, want.PublicKey, got.PublicKey)
		require.Equal(t, want.OS, got.OS)
		require.Equal(t, want.Arch, got.Arch)
		require.Equal(t, want.Cores, got.Cores)
		require.Equal(t, want.Memory, got.Memory)
		require.Equal(t, want.Disk, got.Disk)
		require.Equal(t, want.Vram, got.Vram)
		require.Equal(t, want.WorkloadTTL, got.WorkloadTTL)
		require.True(t, got.Uploadable)
		require.Equal(t, []string{"gpu", "fast"}, got.LabelList())
		require.Equal(t, []string{"/ip4/192.168.1.2/tcp/4001"}, got.AddrList())
	})

	t.Run("conflict keeps operator fields and refreshes the rest", func(t *testing.T) {
		op := got
		op.Disabled = true
		op.ExpiresAfter = int64(time.Hour)
		op.Description = "renamed by operator"
		require.NoError(t, lan.PeerUpdate(t.Context(), db, op).Scan(&op))

		again := langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"), func(p *lan.Peer) {
			p.Cores = 16
			p.Addrs = lan.EncodeList([]string{"/ip4/192.168.1.9/tcp/4001"})
		})
		var updated lan.Peer
		require.NoError(t, lan.PeerUpsert(t.Context(), db, again).Scan(&updated))

		require.Equal(t, int64(16), updated.Cores)
		require.Equal(t, []string{"/ip4/192.168.1.9/tcp/4001"}, updated.AddrList())
		require.True(t, updated.Disabled, "disabled must survive a sighting")
		require.Equal(t, int64(time.Hour), updated.ExpiresAfter, "ttl override must survive a sighting")
		require.True(t, updated.FirstSeen.Equal(got.FirstSeen), "first_seen must not change")
		require.False(t, updated.LastSeen.Before(got.LastSeen))
	})
}
