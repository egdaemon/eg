package eggithub

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaleReleases(t *testing.T) {
	t.Run("everything beyond the newest keep published releases is returned", func(t *testing.T) {
		listing := `[
			{"tagName":"r2","isDraft":false,"createdAt":"2026-09-01T12:00:00Z"},
			{"tagName":"r4","isDraft":false,"createdAt":"2026-11-01T12:00:00Z"},
			{"tagName":"r1","isDraft":false,"createdAt":"2026-08-01T12:00:00Z"},
			{"tagName":"r3","isDraft":false,"createdAt":"2026-10-01T12:00:00Z"}
		]`

		tags, err := staleReleases(listing, 2)
		require.NoError(t, err)
		require.Equal(t, []string{"r2", "r1"}, tags)
	})

	t.Run("drafts are neither returned nor counted", func(t *testing.T) {
		listing := `[
			{"tagName":"draft","isDraft":true,"createdAt":"2026-11-02T12:00:00Z"},
			{"tagName":"new","isDraft":false,"createdAt":"2026-11-01T12:00:00Z"},
			{"tagName":"old","isDraft":false,"createdAt":"2026-08-01T12:00:00Z"}
		]`

		tags, err := staleReleases(listing, 1)
		require.NoError(t, err)
		require.Equal(t, []string{"old"}, tags)
	})

	t.Run("fewer releases than keep has nothing stale", func(t *testing.T) {
		listing := `[{"tagName":"r1","isDraft":false,"createdAt":"2026-08-01T12:00:00Z"}]`

		tags, err := staleReleases(listing, defaultReleaseKeep)
		require.NoError(t, err)
		require.Empty(t, tags)
	})

	t.Run("an empty listing has no stale releases", func(t *testing.T) {
		tags, err := staleReleases("[]", defaultReleaseKeep)
		require.NoError(t, err)
		require.Empty(t, tags)
	})

	t.Run("invalid json is an error", func(t *testing.T) {
		_, err := staleReleases("not json", defaultReleaseKeep)
		require.Error(t, err)
	})
}
