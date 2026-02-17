package egdart_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/x/wasi/egdart"
	"github.com/stretchr/testify/require"
)

func TestCacheDirectory(t *testing.T) {
	t.Run("returns_default_path_when_no_dirs_are_provided", func(t *testing.T) {
		result := egdart.CacheDirectory()
		expected := egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "dart", "")
		require.Equal(t, expected, result)
	})

	t.Run("returns_path_with_single_directory", func(t *testing.T) {
		result := egdart.CacheDirectory("pub-cache")
		expected := egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "dart", "pub-cache")
		require.Equal(t, expected, result)
	})

	t.Run("returns_path_with_multiple_directories", func(t *testing.T) {
		result := egdart.CacheDirectory("my", "nested", "cache")
		expected := egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "dart", filepath.Join("my", "nested", "cache"))
		require.Equal(t, expected, result)
	})

	t.Run("returns_path_with_empty_string_dir", func(t *testing.T) {
		result := egdart.CacheDirectory("")
		expected := egenv.CacheDirectory(_eg.DefaultModuleDirectory(), "dart", "")
		require.Equal(t, expected, result)
	})
}

func TestEnv(t *testing.T) {
	t.Run("returns_environment_with_pub_cache", func(t *testing.T) {
		env := egdart.Env()
		require.Contains(t, env, fmt.Sprintf("PUB_CACHE=%s", egdart.CacheDirectory("pub-cache")))
	})

	t.Run("does_not_preserve_existing_environment", func(t *testing.T) {
		env := egdart.Env()
		for _, v := range env {
			if len(v) > 5 && v[:5] == "PATH=" {
				t.Fatal("expected PATH to not be present in environment")
			}
		}
	})
}

func TestRuntime(t *testing.T) {
	t.Run("returns_a_command", func(t *testing.T) {
		runtime := egdart.Runtime()
		require.NotZero(t, runtime)
	})
}

func TestFindRoots(t *testing.T) {
	t.Run("finds_pubspec_yaml_in_directory_tree", func(t *testing.T) {
		dir := t.TempDir()

		// create a nested project structure
		projDir := filepath.Join(dir, "myapp")
		require.NoError(t, os.MkdirAll(projDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(projDir, "pubspec.yaml"), []byte("name: myapp"), 0644))

		// create a second nested project
		subDir := filepath.Join(dir, "packages", "subpkg")
		require.NoError(t, os.MkdirAll(subDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "pubspec.yaml"), []byte("name: subpkg"), 0644))

		var found []string
		for path := range egdart.FindRoots(dir) {
			found = append(found, path)
		}

		require.Len(t, found, 2)
		require.Contains(t, found, filepath.Join(projDir, "pubspec.yaml"))
		require.Contains(t, found, filepath.Join(subDir, "pubspec.yaml"))
	})

	t.Run("ignores_hidden_directories", func(t *testing.T) {
		dir := t.TempDir()

		// create a hidden directory with a pubspec
		hiddenDir := filepath.Join(dir, ".dart_tool")
		require.NoError(t, os.MkdirAll(hiddenDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(hiddenDir, "pubspec.yaml"), []byte("name: hidden"), 0644))

		// create a visible project
		projDir := filepath.Join(dir, "myapp")
		require.NoError(t, os.MkdirAll(projDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(projDir, "pubspec.yaml"), []byte("name: myapp"), 0644))

		var found []string
		for path := range egdart.FindRoots(dir) {
			found = append(found, path)
		}

		require.Len(t, found, 1)
		require.Contains(t, found, filepath.Join(projDir, "pubspec.yaml"))
	})

	t.Run("returns_nothing_for_empty_directory", func(t *testing.T) {
		dir := t.TempDir()

		var found []string
		for path := range egdart.FindRoots(dir) {
			found = append(found, path)
		}

		require.Empty(t, found)
	})
}
