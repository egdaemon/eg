package eggithub

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyAssets(t *testing.T) {
	touch := func(t *testing.T, dir string, names ...string) {
		t.Helper()
		for _, n := range names {
			require.NoError(t, os.WriteFile(filepath.Join(dir, n), []byte("x"), 0600))
		}
	}

	t.Run("nil when every matched file is present in the asset listing", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "eg_amd64.deb", "eg_arm64.deb")

		err := verifyAssets("r1", "eg_amd64.deb\neg_arm64.deb\n", filepath.Join(dir, "*.deb"))
		require.NoError(t, err)
	})

	t.Run("error when a matched file is missing from the asset listing", func(t *testing.T) {
		dir := t.TempDir()
		touch(t, dir, "eg_amd64.deb")

		err := verifyAssets("r1", "some-other-file.txt\n", filepath.Join(dir, "*.deb"))
		require.ErrorContains(t, err, "r1")
		require.ErrorContains(t, err, "eg_amd64.deb")
	})

	t.Run("error when the pattern matches no local files", func(t *testing.T) {
		dir := t.TempDir()

		err := verifyAssets("r1", "eg_amd64.deb\n", filepath.Join(dir, "*.deb"))
		require.ErrorContains(t, err, "r1")
	})

	t.Run("no patterns is a no-op", func(t *testing.T) {
		err := verifyAssets("r1", "")
		require.NoError(t, err)
	})

	t.Run("examples", func(t *testing.T) {
		assert.NoError(t, verifyAssets("", "eg_0.0.1786975627107_amd64.deb", "eg_*_amd64.deb"))
		assert.NoError(t, verifyAssets("", "eg_0.0.1786975627107_arm64.deb", "eg_*_arm64.deb"))
		assert.NoError(t, verifyAssets("", "eg.dmg", "eg.dmg"))
		assert.NoError(t, verifyAssets("", "eg_0.0.1_amd64.deb\neg_0.0.1_arm64.deb\neg.dmg", "eg_*_amd64.deb"))
		assert.NoError(t, verifyAssets("", "eg_0.0.1_amd64.deb\neg_0.0.1_arm64.deb\neg.dmg", "eg_*_amd64.deb", "eg_*_arm64.deb", "eg.dmg"))
		assert.NoError(t, verifyAssets("", "eg_0.0.1_amd64.deb\n\neg_0.0.1_arm64.deb\n", "eg_*_amd64.deb", "eg_*_arm64.deb"))

		assert.Error(t, verifyAssets("r2026.8.1", "eg_0.0.1_amd64.deb", "eg_*_arm64.deb"))
		assert.Error(t, verifyAssets("", "eg_0.0.1_amd64.deb", "eg.dmg"))
		assert.Error(t, verifyAssets("", "", "eg_*_amd64.deb"))
		assert.Error(t, verifyAssets("", "EG_0.0.1_AMD64.DEB", "eg_*_amd64.deb"))
		assert.Error(t, verifyAssets("", "eg_0.0.1_amd64.deb.sig", "eg_*_amd64.deb"))
		assert.Error(t, verifyAssets("", "eg_0.0.1_amd64.deb\neg_0.0.1_arm64.deb", "eg_*_amd64.deb", "eg.dmg"))
	})
}
