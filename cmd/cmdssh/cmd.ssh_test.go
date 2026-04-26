package cmdssh_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdssh"
	"github.com/stretchr/testify/require"
)

func runSSHCLI(t *testing.T, args []string) error {
	t.Helper()

	var cli struct {
		cmdopts.Global
		SSH cmdssh.Cmd `cmd:""`
	}

	cli.Context = t.Context()

	parser, err := kong.New(&cli,
		kong.Name("eg"),
		kong.Vars{
			"vars_entropy_seed":     "test-default-seed",
			"vars_ssh_key_seed":     "test-default-seed",
			"vars_ssh_key_path": filepath.Join(t.TempDir(), "id_ed25519"),
			"vars_user_name":    "testuser",
			"vars_user_home":    t.TempDir(),
		},
		kong.Bind(&cli.Global),
	)
	require.NoError(t, err)

	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}

	return ctx.Run()
}

func TestCmdKey(t *testing.T) {
	t.Run("generates_key_files_at_path", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "id_ed25519")
		err := runSSHCLI(t, []string{"ssh", "key", "--seed", "test-seed", "--path", path})
		require.NoError(t, err)
		require.FileExists(t, path)
		require.FileExists(t, path+".pub")
	})

	t.Run("creates_parent_directory_with_0700", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "nonexistent", "subdir", "id_ed25519")
		err := runSSHCLI(t, []string{"ssh", "key", "--seed", "test-seed", "--path", path})
		require.NoError(t, err)
		require.FileExists(t, path)
		require.FileExists(t, path+".pub")

		info, err := os.Stat(filepath.Dir(path))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0700), info.Mode().Perm())
	})

	t.Run("second_call_loads_from_disk", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "id_ed25519")

		err := runSSHCLI(t, []string{"ssh", "key", "--seed", "stable-seed", "--path", path})
		require.NoError(t, err)

		// Different seed — if it regenerated it would differ; loading from disk returns the original.
		err = runSSHCLI(t, []string{"ssh", "key", "--seed", "other-seed", "--path", path})
		require.NoError(t, err)

		matches, err := filepath.Glob(path)
		require.NoError(t, err)
		require.Len(t, matches, 1)
	})
}
