package egccache_test

import (
	"fmt"
	"path/filepath"
	"testing"

	_eg "github.com/egdaemon/eg"

	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/x/wasi/egccache"

	"github.com/stretchr/testify/require"
)

func TestCacheDirectory(t *testing.T) {
	t.Run("CacheDirectory", func(t *testing.T) {
		t.Run("returns_default_path_when_no_dirs_are_provided", func(t *testing.T) {
			result := egccache.CacheDirectory()
			expected := egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "ccache", "")
			require.Equal(t, expected, result)
		})

		t.Run("returns_path_with_single_directory", func(t *testing.T) {
			result := egccache.CacheDirectory("mycache")
			expected := egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "ccache", "mycache")
			require.Equal(t, expected, result)
		})

		t.Run("returns_path_with_multiple_directories", func(t *testing.T) {
			result := egccache.CacheDirectory("my", "nested", "cache")
			expected := egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "ccache", filepath.Join("my", "nested", "cache"))
			require.Equal(t, expected, result)
		})

		t.Run("returns_path_with_empty_string_dir", func(t *testing.T) {
			result := egccache.CacheDirectory("")
			expected := egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "ccache", "")
			require.Equal(t, expected, result)
		})

		t.Run("returns_path_with_absolute_path", func(t *testing.T) {
			result := egccache.CacheDirectory("/var/tmp/ccache")
			expected := egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "ccache", "/var/tmp/ccache")
			require.Equal(t, expected, result)
		})
	})
}

func TestOptions(t *testing.T) {
	t.Run("Options", func(t *testing.T) {
		t.Run("Env_includes_default_ccache_dir", func(t *testing.T) {
			opts := egccache.Options()
			env := opts.Env()
			require.Contains(t, env, fmt.Sprintf("CCACHE_DIR=%s", egccache.CacheDirectory()))
		})

		t.Run("Log_adds_ccache_logfile_variable", func(t *testing.T) {
			logPath := "/var/log/ccache.log"
			opts := egccache.Options().Log(logPath)
			env := opts.Env()
			require.Contains(t, env, fmt.Sprintf("CCACHE_LOGFILE=%s", logPath))
			require.Contains(t, env, fmt.Sprintf("CCACHE_DIR=%s", egccache.CacheDirectory()))
		})

		t.Run("MaxDisk_adds_ccache_maxsize_variable", func(t *testing.T) {
			size := uint64(5 * bytesx.MiB)
			opts := egccache.Options().MaxDisk(size)
			env := opts.Env()
			require.Contains(t, env, fmt.Sprintf("CCACHE_MAXSIZE=%X", bytesx.Unit(size)))
			require.Contains(t, env, fmt.Sprintf("CCACHE_DIR=%s", egccache.CacheDirectory()))
		})

		t.Run("Disable_adds_ccache_disable_variable", func(t *testing.T) {
			opts := egccache.Options().Disable()
			env := opts.Env()
			require.Contains(t, env, "CCACHE_DISABLE=1")
			require.Contains(t, env, fmt.Sprintf("CCACHE_DIR=%s", egccache.CacheDirectory()))
		})

		t.Run("Recache_adds_ccache_recache_variable", func(t *testing.T) {
			opts := egccache.Options().Recache()
			env := opts.Env()
			require.Contains(t, env, "CCACHE_RECACHE=1")
			require.Contains(t, env, fmt.Sprintf("CCACHE_DIR=%s", egccache.CacheDirectory()))
		})

		t.Run("combines_multiple_options", func(t *testing.T) {
			logPath := "/tmp/combined.log"
			size := uint64(5 * bytesx.MiB)
			opts := egccache.Options().Log(logPath).MaxDisk(size).Disable().Recache()
			env := opts.Env()

			require.Contains(t, env, fmt.Sprintf("CCACHE_DIR=%s", egccache.CacheDirectory()))
			require.Contains(t, env, fmt.Sprintf("CCACHE_LOGFILE=%s", logPath))
			require.Contains(t, env, fmt.Sprintf("CCACHE_MAXSIZE=%X", bytesx.Unit(size)))
			require.Contains(t, env, "CCACHE_DISABLE=1")
			require.Contains(t, env, "CCACHE_RECACHE=1")
		})
	})
}

func TestEnv(t *testing.T) {
	t.Run("Env", func(t *testing.T) {
		t.Run("returns_default_ccache_environment", func(t *testing.T) {
			env := egccache.Env()
			require.Contains(t, env, fmt.Sprintf("CCACHE_DIR=%s", egccache.CacheDirectory()))
			require.NotContains(t, env, "CCACHE_LOGFILE")
			require.NotContains(t, env, "CCACHE_MAXSIZE")
			require.NotContains(t, env, "CCACHE_DISABLE")
			require.NotContains(t, env, "CCACHE_RECACHE")
		})
	})
}
