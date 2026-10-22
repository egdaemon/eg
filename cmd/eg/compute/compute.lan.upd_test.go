package compute

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdtestx"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestLanUpdate(t *testing.T) {
	gctx := &cmdopts.Global{Context: t.Context()}

	t.Run("disable and enable", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"))).Scan(&lan.Peer{}))

		p, err := lanUpdate{PeerID: "a", Disable: true}.update(gctx, db)
		require.NoError(t, err)
		require.True(t, p.Disabled)

		p, err = lanUpdate{PeerID: "a", Enable: true}.update(gctx, db)
		require.NoError(t, err)
		require.False(t, p.Disabled)
	})

	t.Run("ttl", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"))).Scan(&lan.Peer{}))

		p, err := lanUpdate{PeerID: "a", TTL: 2 * time.Hour}.update(gctx, db)
		require.NoError(t, err)
		require.Equal(t, int64(2*time.Hour), p.ExpiresAfter)

		p, err = lanUpdate{PeerID: "a", TTL: time.Duration(math.MaxInt)}.update(gctx, db)
		require.NoError(t, err)
		require.Equal(t, int64(-1), p.ExpiresAfter, "infinity never expires")

		p, err = lanUpdate{PeerID: "a", ResetTTL: true}.update(gctx, db)
		require.NoError(t, err)
		require.Zero(t, p.ExpiresAfter, "reset restores the default")
	})

	t.Run("description keeps everything else", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"))).Scan(&lan.Peer{}))

		desc := "the build box"
		p, err := lanUpdate{PeerID: "a", Description: &desc, Disable: true}.update(gctx, db)
		require.NoError(t, err)
		require.Equal(t, desc, p.Description)
		require.True(t, p.Disabled)
		require.Equal(t, int64(8), p.Cores)
	})

	t.Run("requires a change", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"))).Scan(&lan.Peer{}))

		_, err := lanUpdate{PeerID: "a"}.update(gctx, db)
		require.ErrorContains(t, err, "nothing to update")
	})

	t.Run("unknown system", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))

		_, err := lanUpdate{PeerID: "missing", Disable: true}.update(gctx, db)
		require.ErrorContains(t, err, "missing")
	})

	t.Run("runs against the given directory", func(t *testing.T) {
		dir := t.TempDir()

		db := testx.MustT(lan.Open(t.Context(), filepath.Join(dir, migrations.Runner), lan.DefaultTTL))(t)
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"))).Scan(&lan.Peer{}))
		require.NoError(t, db.Close())

		genparser := cmdtestx.Genparser(lanUpdate{}, kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)), kong.Vars{"vars_cache_directory": t.TempDir()})
		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command", "a", "--runner", "--cache-directory", dir, "--disable", "--ttl", "infinity", "--description", "x"))

		db = testx.MustT(lan.Open(t.Context(), filepath.Join(dir, migrations.Runner), lan.DefaultTTL))(t)
		defer db.Close()

		var p lan.Peer
		require.NoError(t, lan.PeerFindByID(t.Context(), db, "a").Scan(&p))
		require.True(t, p.Disabled)
		require.Equal(t, int64(-1), p.ExpiresAfter)
		require.Equal(t, "x", p.Description)
	})

	t.Run("rejects invalid flags", func(t *testing.T) {
		genparser := cmdtestx.Genparser(lanUpdate{}, kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)), kong.Vars{"vars_cache_directory": t.TempDir()})

		_, err := genparser(t).Parse([]string{"command"})
		require.Error(t, err, "peer id is required")

		_, err = genparser(t).Parse([]string{"command", "a", "--disable", "--enable"})
		require.Error(t, err, "disable and enable are exclusive")

		_, err = genparser(t).Parse([]string{"command", "a", "--ttl", "1h", "--reset-ttl"})
		require.Error(t, err, "ttl and reset-ttl are exclusive")
	})
}
