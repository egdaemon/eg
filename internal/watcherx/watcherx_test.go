package watcherx_test

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/internal/watcherx"
	"github.com/stretchr/testify/require"
)

func atime(t *testing.T, path string) time.Time {
	t.Helper()
	var st syscall.Stat_t
	require.NoError(t, syscall.Stat(path, &st))
	return time.Unix(st.Atim.Sec, st.Atim.Nsec)
}

func setpast(t *testing.T, path string) {
	t.Helper()
	past := time.Now().Add(-time.Hour)
	require.NoError(t, os.Chtimes(path, past, past))
}

// mirror creates a file at the same relative path under both roots.
func mirror(t *testing.T, src, dst, rel string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(src, rel)), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dst, rel)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, rel), nil, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dst, rel), nil, 0o644))
}

func TestProxy(t *testing.T) {
	t.Run("write_touches_corresponding_dst_file", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		src, dst := t.TempDir(), t.TempDir()
		mirror(t, src, dst, "data.txt")
		setpast(t, filepath.Join(dst, "data.txt"))
		before := atime(t, filepath.Join(dst, "data.txt"))

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() { watcherx.Proxy(ctx, src, dst, 10*time.Millisecond) }()

		time.Sleep(50 * time.Millisecond)
		require.NoError(t, os.WriteFile(filepath.Join(src, "data.txt"), []byte("updated"), 0o644))

		require.Eventually(t, func() bool {
			return atime(t, filepath.Join(dst, "data.txt")).After(before)
		}, 2*time.Second, 20*time.Millisecond)
	})

	t.Run("write_in_subdir_touches_corresponding_dst_file", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		src, dst := t.TempDir(), t.TempDir()
		mirror(t, src, dst, "subdir/nested.txt")
		setpast(t, filepath.Join(dst, "subdir", "nested.txt"))
		before := atime(t, filepath.Join(dst, "subdir", "nested.txt"))

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() { watcherx.Proxy(ctx, src, dst, 10*time.Millisecond) }()

		time.Sleep(50 * time.Millisecond)
		require.NoError(t, os.WriteFile(filepath.Join(src, "subdir", "nested.txt"), []byte("change"), 0o644))

		require.Eventually(t, func() bool {
			return atime(t, filepath.Join(dst, "subdir", "nested.txt")).After(before)
		}, 2*time.Second, 20*time.Millisecond)
	})

	t.Run("remove_touches_dst_parent_directory", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		src, dst := t.TempDir(), t.TempDir()
		mirror(t, src, dst, "gone.txt")
		setpast(t, dst)
		before := atime(t, dst)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			if err := watcherx.Proxy(ctx, src, dst, 10*time.Millisecond); err != nil {
				t.Logf("Proxy error: %v", err)
			}
		}()

		time.Sleep(50 * time.Millisecond)
		require.NoError(t, os.Remove(filepath.Join(src, "gone.txt")))

		require.Eventually(t, func() bool {
			return atime(t, dst).After(before)
		}, 2*time.Second, 20*time.Millisecond)
	})

	t.Run("context_cancellation_stops_proxy", func(t *testing.T) {
		ctx, done := testx.Context(t)
		defer done()

		ctx, cancel := context.WithCancel(ctx)
		errc := make(chan error, 1)
		go func() { errc <- watcherx.Proxy(ctx, t.TempDir(), t.TempDir(), 10*time.Millisecond) }()

		time.Sleep(20 * time.Millisecond)
		cancel()

		select {
		case err := <-errc:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(2 * time.Second):
			t.Fatal("Proxy did not stop after context cancellation")
		}
	})
}
