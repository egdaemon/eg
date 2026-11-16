package cmdgpg_test

import (
	"github.com/egdaemon/eg"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdgpg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/gpgx"
	"github.com/stretchr/testify/require"
)

func runGPGCLI(t *testing.T, args []string) error {
	t.Helper()

	var cli struct {
		cmdopts.Global
		GPG cmdgpg.Cmd `cmd:""`
	}

	cli.Context = t.Context()

	parser, err := kong.New(&cli,
		kong.Name("eg"),
		kong.Vars{
			"vars_entropy_seed":  "test-default-seed",
			"vars_gpg_directory": t.TempDir(),
			"vars_user_name":     "testuser",
			"env_eg_account":     eg.EnvAccount,
			"env_eg_profile":     eg.EnvProfile,

			"env_eg_gpg_keyring_home":  eg.EnvGPGKeyringHome,
			"env_eg_gpg_keyring_name":  eg.EnvGPGKeyringName,
			"env_eg_gpg_keyring_email": eg.EnvGPGKeyringEmail,
			"env_eg_gpg_keyring_seed":  eg.EnvGPGKeyringSeed,
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

	t.Run("reads_the_eggpg_environment_variables", func(t *testing.T) {
		flagdir := t.TempDir()
		err := runGPGCLI(t, []string{
			"gpg", "keyring",
			"--seed", "env-seed",
			"--directory", flagdir,
			"--name", "Engineering",
			"--email", "engineering@example.com",
		})
		require.NoError(t, err)

		envdir := t.TempDir()
		t.Setenv(eg.EnvGPGKeyringSeed, "env-seed")
		t.Setenv(eg.EnvGPGKeyringHome, envdir)
		t.Setenv(eg.EnvGPGKeyringName, "Engineering")
		t.Setenv(eg.EnvGPGKeyringEmail, "engineering@example.com")
		require.NoError(t, runGPGCLI(t, []string{"gpg", "keyring"}))

		// the self signatures carry timestamps so the armor differs, the fingerprint is what the seed determines.
		expected, err := gpgx.Keyring(flagdir, "unused")
		require.NoError(t, err)
		actual, err := gpgx.Keyring(envdir, "unused")
		require.NoError(t, err)
		require.Equal(t, expected.PrimaryKey.Fingerprint, actual.PrimaryKey.Fingerprint)
	})

	t.Run("flags_take_precedence_over_the_environment", func(t *testing.T) {
		envdir := t.TempDir()
		flagdir := t.TempDir()
		t.Setenv(eg.EnvGPGKeyringSeed, "env-seed")
		t.Setenv(eg.EnvGPGKeyringHome, envdir)
		require.NoError(t, runGPGCLI(t, []string{"gpg", "keyring", "--directory", flagdir, "--email", "test@example.com"}))
		require.FileExists(t, filepath.Join(flagdir, "private.asc"))
		require.NoFileExists(t, filepath.Join(envdir, "private.asc"))
	})

	t.Run("prints_the_eggpg_diagnostics_to_stderr_with_the_seed_redacted", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv(eg.EnvGPGKeyringSeed, "super-secret-seed")
		stderr := os.Stderr
		r, w, err := os.Pipe()
		require.NoError(t, err)
		os.Stderr = w
		runerr := runGPGCLI(t, []string{"gpg", "keyring", "--directory", dir, "--email", "test@example.com"})
		os.Stderr = stderr
		require.NoError(t, w.Close())
		require.NoError(t, runerr)

		out, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Contains(t, string(out), "# env | grep -i EG_GPG_")
		require.Contains(t, string(out), eg.EnvGPGKeyringSeed+"=<redacted>")
		require.Contains(t, string(out), "# sha256sum private.asc public.asc")
		require.Contains(t, string(out), filepath.Join(dir, "private.asc"))
		require.NotContains(t, string(out), "super-secret-seed")
		require.NotContains(t, string(out), "PRIVATE KEY BLOCK")
	})
}
