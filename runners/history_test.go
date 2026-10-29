package runners

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHistoryRecorderKeepLogs(t *testing.T) {
	t.Run("links the log so it outlives the workload's directory", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "daemon.log")
		require.NoError(t, os.WriteFile(src, []byte("output"), 0600))

		path, err := historyrecorder{logs: filepath.Join(t.TempDir(), "history")}.keeplogs("w1", src)
		require.NoError(t, err)

		srcinfo, err := os.Stat(src)
		require.NoError(t, err)
		dstinfo, err := os.Stat(path)
		require.NoError(t, err)
		require.True(t, os.SameFile(srcinfo, dstinfo), "linked, not copied")

		require.NoError(t, os.RemoveAll(filepath.Dir(src)), "the workload directory is wiped")
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "output", string(b))
	})

	t.Run("a repeated completion replaces the earlier log", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "history")
		first := filepath.Join(t.TempDir(), "first.log")
		second := filepath.Join(t.TempDir(), "second.log")
		require.NoError(t, os.WriteFile(first, []byte("first"), 0600))
		require.NoError(t, os.WriteFile(second, []byte("second"), 0600))

		_, err := historyrecorder{logs: dir}.keeplogs("w1", first)
		require.NoError(t, err)
		path, err := historyrecorder{logs: dir}.keeplogs("w1", second)
		require.NoError(t, err)

		b, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "second", string(b))
	})

	t.Run("a missing log is an error", func(t *testing.T) {
		_, err := historyrecorder{logs: t.TempDir()}.keeplogs("w1", filepath.Join(t.TempDir(), "missing.log"))
		require.Error(t, err)
	})
}

func TestHistoryRecorderKeepAnalytics(t *testing.T) {
	t.Run("links the analytics so they outlive the workload's directory", func(t *testing.T) {
		src := filepath.Join(t.TempDir(), "analytics.db")
		require.NoError(t, os.WriteFile(src, []byte("analytics"), 0600))

		path, err := historyrecorder{logs: filepath.Join(t.TempDir(), "history")}.keepanalytics("w1", src)
		require.NoError(t, err)
		require.Equal(t, "w1.analytics.db", filepath.Base(path))

		require.NoError(t, os.RemoveAll(filepath.Dir(src)), "the workload directory is wiped")
		b, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "analytics", string(b))
	})

	t.Run("missing analytics report not exist", func(t *testing.T) {
		_, err := historyrecorder{logs: t.TempDir()}.keepanalytics("w1", filepath.Join(t.TempDir(), "missing.db"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}
