package gpgx_test

import (
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/gpgx"
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
}
