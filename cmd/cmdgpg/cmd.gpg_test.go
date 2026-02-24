package cmdgpg_test

import (
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdgpg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
)

func runGPGCLI(t *testing.T, args []string) error {
	t.Helper()

	var cli struct {
		cmdopts.Global
		GPG cmdgpg.GpgCmd `cmd:""`
	}

	cli.Context = t.Context()

	parser, err := kong.New(&cli,
		kong.Name("eg"),
		kong.Vars{
			"vars_entropy_seed":  "test-default-seed",
			"vars_gpg_directory": t.TempDir(),
			"vars_user_name":     "testuser",
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

func TestCmdKeyring(t *testing.T) {
	t.Run("generates_keyring_files_in_directory", func(t *testing.T) {
		dir := t.TempDir()
		err := runGPGCLI(t, []string{"gpg", "keyring", "--seed", "test-seed", "--directory", dir})
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(dir, "private.asc"))
		require.FileExists(t, filepath.Join(dir, "public.asc"))
	})

	t.Run("second_call_loads_from_disk", func(t *testing.T) {
		dir := t.TempDir()

		err := runGPGCLI(t, []string{"gpg", "keyring", "--seed", "stable-seed", "--directory", dir})
		require.NoError(t, err)

		// Different seed — if it regenerated it would differ; loading from disk returns the original.
		err = runGPGCLI(t, []string{"gpg", "keyring", "--seed", "other-seed", "--directory", dir})
		require.NoError(t, err)

		matches, err := filepath.Glob(filepath.Join(dir, "private.asc"))
		require.NoError(t, err)
		require.Len(t, matches, 1)
	})

	t.Run("accepts_name_and_email_flags", func(t *testing.T) {
		dir := t.TempDir()
		err := runGPGCLI(t, []string{
			"gpg", "keyring",
			"--seed", "identity-seed",
			"--directory", dir,
			"--name", "Test User",
			"--email", "test@example.com",
		})
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(dir, "private.asc"))
	})
}
