package gpgx_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/gpgx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestKeyGen_Generate(t *testing.T) {
	t.Run("returns_valid_entity", func(t *testing.T) {
		entity, err := gpgx.NewKeyGenSeeded("test-seed").Generate()
		require.NoError(t, err)
		require.NotNil(t, entity)
		require.NotNil(t, entity.PrimaryKey)
		require.True(t, entity.PrimaryKey.CanSign())
	})

	t.Run("same_seed_same_key_id", func(t *testing.T) {
		e1, err := gpgx.NewKeyGenSeeded("deterministic").Generate()
		require.NoError(t, err)

		e2, err := gpgx.NewKeyGenSeeded("deterministic").Generate()
		require.NoError(t, err)

		require.Equal(t, e1.PrimaryKey.KeyId, e2.PrimaryKey.KeyId)
	})

	t.Run("different_seeds_different_key_ids", func(t *testing.T) {
		e1, err := gpgx.NewKeyGenSeeded("seed-alpha").Generate()
		require.NoError(t, err)

		e2, err := gpgx.NewKeyGenSeeded("seed-beta").Generate()
		require.NoError(t, err)

		require.NotEqual(t, e1.PrimaryKey.KeyId, e2.PrimaryKey.KeyId)
	})
}

func TestKeyring(t *testing.T) {
	t.Run("generates_and_writes_keyring_files", func(t *testing.T) {
		dir := t.TempDir()
		_, err := gpgx.Keyring(dir, "test-seed")
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(dir, "private.asc"))
		require.FileExists(t, filepath.Join(dir, "public.asc"))
	})

	t.Run("same_seed_same_key_id_across_calls", func(t *testing.T) {
		dir := t.TempDir()

		e1, err := gpgx.Keyring(dir, "cache-seed")
		require.NoError(t, err)

		e2, err := gpgx.Keyring(dir, "cache-seed")
		require.NoError(t, err)

		require.Equal(t, e1.PrimaryKey.KeyId, e2.PrimaryKey.KeyId)
	})

	t.Run("second_call_loads_from_disk", func(t *testing.T) {
		dir := t.TempDir()

		_, err := gpgx.Keyring(dir, "disk-seed")
		require.NoError(t, err)

		// Use a different seed — if the second call generates fresh it would differ;
		// loading from disk should return the original key.
		e2, err := gpgx.Keyring(dir, "different-seed")
		require.NoError(t, err)

		e1, err := gpgx.NewKeyGenSeeded("disk-seed").Generate()
		require.NoError(t, err)

		require.Equal(t, e1.PrimaryKey.KeyId, e2.PrimaryKey.KeyId)
	})

	t.Run("integration with gpg import", func(t *testing.T) {
		gpgpath, err := execx.LookPath("gpg")
		if err != nil {
			t.Skip("gpg not available:", err)
		}

		gnuhome := t.TempDir()

		env := os.Environ()
		env = append(env, "GNUPGHOME="+gnuhome)
		runOnce := func(seed string) {
			dir := t.TempDir()
			_, err := gpgx.Keyring(dir, seed)
			require.NoError(t, err)

			var out bytes.Buffer
			cmd := exec.CommandContext(t.Context(), gpgpath, "--import", filepath.Join(dir, "private.asc"))
			cmd.Env = env
			cmd.Stdout = &out
			cmd.Stderr = &out

			err = execx.MaybeRun(cmd)
			require.NoErrorf(t, err, "seed=%s output=%s", seed, out.String())
		}

		for range envx.Int(512, "GPGX_FUZZ_N") {
			runOnce(uuid.Must(uuid.NewV7()).String())
		}

		if seed := envx.String("", "GPGX_FUZZ_SEED"); stringsx.Present(seed) {
			runOnce(seed)
		}
	})
}
