package cmdssh_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg"
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
			"vars_entropy_seed":   "test-default-seed",
			"vars_ssh_key_seed":   "test-default-seed",
			"env_eg_ssh_key_seed": eg.EnvEGSSHSeed,
			"vars_ssh_key_path":   filepath.Join(t.TempDir(), "id_ed25519"),
			"vars_user_name":      "testuser",
			"vars_user_home":      t.TempDir(),
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

	t.Run("seed falls_back_to_the_ssh_key_seed_environment", func(t *testing.T) {
		t.Setenv(eg.EnvEGSSHSeed, "env-seed")
		dir := t.TempDir()
		envpath := filepath.Join(dir, "env_ed25519")
		argpath := filepath.Join(dir, "arg_ed25519")

		require.NoError(t, runSSHCLI(t, []string{"ssh", "key", "--path", envpath}))
		require.NoError(t, runSSHCLI(t, []string{"ssh", "key", "--seed", "env-seed", "--path", argpath}))

		envpub, err := os.ReadFile(envpath + ".pub")
		require.NoError(t, err)
		argpub, err := os.ReadFile(argpath + ".pub")
		require.NoError(t, err)
		require.Equal(t, argpub, envpub)
	})

	t.Run("explicit_seed_wins_over_the_ssh_key_seed_environment", func(t *testing.T) {
		t.Setenv(eg.EnvEGSSHSeed, "env-seed")
		dir := t.TempDir()
		envpath := filepath.Join(dir, "env_ed25519")
		argpath := filepath.Join(dir, "arg_ed25519")

		require.NoError(t, runSSHCLI(t, []string{"ssh", "key", "--path", envpath}))
		require.NoError(t, runSSHCLI(t, []string{"ssh", "key", "--seed", "arg-seed", "--path", argpath}))

		envpub, err := os.ReadFile(envpath + ".pub")
		require.NoError(t, err)
		argpub, err := os.ReadFile(argpath + ".pub")
		require.NoError(t, err)
		require.NotEqual(t, argpub, envpub)
	})
}
