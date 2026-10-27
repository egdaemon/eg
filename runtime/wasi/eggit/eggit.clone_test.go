package eggit

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/stretchr/testify/require"
)

func TestClone(t *testing.T) {
	t.Run("blank uri", func(t *testing.T) {
		err := Clone(t.Context(), " ", "origin", "main")
		require.ErrorContains(t, err, "url not specified")
	})

	t.Run("blank remote", func(t *testing.T) {
		err := Clone(t.Context(), "https://example.com/repo.git", "\t", "main")
		require.ErrorContains(t, err, "remote not specified")
	})

	t.Run("host call unavailable outside of wasi", func(t *testing.T) {
		err := Clone(t.Context(), "https://example.com/repo.git", "origin", "main")
		require.Error(t, err)
	})

	t.Run("auto clone without uri", func(t *testing.T) {
		testx.Tempenvvar(_eg.EnvGitHeadURI, "", func() {
			err := AutoClone(t.Context(), nil)
			require.ErrorContains(t, err, "url not specified")
		})
	})
}
