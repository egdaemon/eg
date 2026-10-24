package generations

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublish(t *testing.T) {
	nochown := func(string) error { return nil }

	write := func(content string) func(string) (bool, error) {
		return func(dir string) (bool, error) {
			return true, os.WriteFile(filepath.Join(dir, "file"), []byte(content), 0600)
		}
	}

	t.Run("should make the generation current", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "eg")
		ok, err := Publish(t.Context(), base, nochown, write("first"))
		require.NoError(t, err)
		require.True(t, ok)

		encoded, err := os.ReadFile(filepath.Join(base, Current, "file"))
		require.NoError(t, err)
		require.Equal(t, "first", string(encoded))
	})

	t.Run("should publish nothing when there is nothing to build", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "eg")
		ok, err := Publish(t.Context(), base, nochown, func(string) (bool, error) { return false, nil })
		require.NoError(t, err)
		require.False(t, ok)
		require.NoFileExists(t, filepath.Join(base, Current))

		entries, err := os.ReadDir(filepath.Join(base, Past))
		require.NoError(t, err)
		require.Empty(t, entries, "expected the unpublished generation to be removed")
	})

	t.Run("should keep the previous generation and remove older ones", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "eg")
		for _, content := range []string{"a", "b", "c", "d"} {
			_, err := Publish(t.Context(), base, nochown, write(content))
			require.NoError(t, err)
		}

		matches, err := filepath.Glob(filepath.Join(base, Past, "*"))
		require.NoError(t, err)
		require.Len(t, matches, 2)

		encoded, err := os.ReadFile(filepath.Join(base, Current, "file"))
		require.NoError(t, err)
		require.Equal(t, "d", string(encoded))
	})

	t.Run("should never expose a partial generation", func(t *testing.T) {
		base := filepath.Join(t.TempDir(), "eg")
		_, err := Publish(t.Context(), base, nochown, write(strings.Repeat("a", 1<<16)))
		require.NoError(t, err)

		done := make(chan struct{})
		var torn atomic.Bool
		var readers sync.WaitGroup
		readers.Add(1)
		go func() {
			defer readers.Done()
			for {
				select {
				case <-done:
					return
				default:
				}

				encoded, err := os.ReadFile(filepath.Join(base, Current, "file"))
				if err != nil || len(encoded) != 1<<16 || len(strings.Trim(string(encoded), string(encoded[:1]))) != 0 {
					torn.Store(true)
				}
			}
		}()

		for i := 0; i < 50; i++ {
			_, err := Publish(t.Context(), base, nochown, write(strings.Repeat(string(rune('a'+i%26)), 1<<16)))
			require.NoError(t, err)
		}

		close(done)
		readers.Wait()
		require.False(t, torn.Load())
	})
}
