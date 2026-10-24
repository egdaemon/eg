package compute

import (
	"database/sql"
	"path/filepath"
	"testing"

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

func TestLanRemove(t *testing.T) {
	gctx := &cmdopts.Global{Context: t.Context()}

	t.Run("forgets every requested system", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		for _, id := range []string{"a", "b", "c"} {
			require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(id))).Scan(&lan.Peer{}))
		}

		require.NoError(t, lanRemove{PeerIDs: []string{"a", "b"}}.remove(gctx, db))

		require.ErrorIs(t, lan.PeerFindByID(t.Context(), db, "a").Scan(&lan.Peer{}), sql.ErrNoRows)
		require.ErrorIs(t, lan.PeerFindByID(t.Context(), db, "b").Scan(&lan.Peer{}), sql.ErrNoRows)
		require.NoError(t, lan.PeerFindByID(t.Context(), db, "c").Scan(&lan.Peer{}))
	})

	t.Run("reports unknown systems but still forgets the rest", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		require.NoError(t, migrations.Migrate(t.Context(), db))
		require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID("a"))).Scan(&lan.Peer{}))

		err := lanRemove{PeerIDs: []string{"missing", "a"}}.remove(gctx, db)
		require.ErrorContains(t, err, "missing")
		require.ErrorIs(t, lan.PeerFindByID(t.Context(), db, "a").Scan(&lan.Peer{}), sql.ErrNoRows)
	})

	t.Run("runs against the given directory", func(t *testing.T) {
		dir := t.TempDir()

		db := testx.MustT(migrations.Open(t.Context(), filepath.Join(dir, migrations.CLI)))(t)
		for _, id := range []string{"a", "b"} {
			require.NoError(t, lan.PeerUpsert(t.Context(), db, langx.Clone(lan.Peer{}, lan.PeerOptionTestDefaults, lan.PeerOptionID(id))).Scan(&lan.Peer{}))
		}
		require.NoError(t, db.Close())

		genparser := cmdtestx.Genparser(lanRemove{}, kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)), kong.Vars{"vars_cache_directory": t.TempDir()})
		require.NoError(t, cmdtestx.Execute(t, genparser(t), "command", "a", "--database", filepath.Join(dir, migrations.CLI)))

		db = testx.MustT(migrations.Open(t.Context(), filepath.Join(dir, migrations.CLI)))(t)
		defer db.Close()
		require.ErrorIs(t, lan.PeerFindByID(t.Context(), db, "a").Scan(&lan.Peer{}), sql.ErrNoRows)
		require.NoError(t, lan.PeerFindByID(t.Context(), db, "b").Scan(&lan.Peer{}))
	})

	t.Run("requires at least one peer id", func(t *testing.T) {
		genparser := cmdtestx.Genparser(lanRemove{}, kong.NamedMapper("durationinf", kong.MapperFunc(cmdopts.ParseDurationInf)), kong.Vars{"vars_cache_directory": t.TempDir()})

		_, err := genparser(t).Parse([]string{"command"})
		require.Error(t, err)
	})
}
