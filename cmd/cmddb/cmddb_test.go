package cmddb

import (
	"bytes"
	"log"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/stretchr/testify/require"
	"github.com/xo/dburl"
	"github.com/xo/usql/drivers"
	"github.com/xo/usql/drivers/completer"
)

func TestUSQLCompletion(t *testing.T) {
	t.Setenv("RUNTIME_DIRECTORY", t.TempDir())
	registerusql()

	u := &dburl.URL{Driver: usqldriver, DSN: filepath.Join(t.TempDir(), migrations.CLI)}
	db, err := drivers.Open(t.Context(), u, nil, nil)
	require.NoError(t, err)
	defer db.Close()

	complete := func(t *testing.T, line string) []string {
		var errs bytes.Buffer
		c := drivers.NewCompleter(t.Context(), u, db, nil, completer.WithLogger(log.New(&errs, "", 0)))
		require.NotNil(t, c)

		suggestions, _ := c.Do([]rune(line), len(line))
		require.Empty(t, errs.String(), "the completer must only query what duckdb supports")

		completed := make([]string, 0, len(suggestions))
		for _, s := range suggestions {
			completed = append(completed, string(s))
		}

		return completed
	}

	t.Run("completes table names", func(t *testing.T) {
		require.Contains(t, complete(t, "SELECT * from serve_ses"), "sions")
	})

	t.Run("completes after a complete table name", func(t *testing.T) {
		complete(t, "SELECT * FROM serve_sessions")
	})

	t.Run("completes the tables of a meta command", func(t *testing.T) {
		require.Contains(t, complete(t, `\d lan_`), "peers")
	})

	t.Run("completes from the start of a line", func(t *testing.T) {
		complete(t, "")
		complete(t, "SELECT * FROM ")
		complete(t, "SELECT * FROM lan_peers WHERE ")
	})
}
