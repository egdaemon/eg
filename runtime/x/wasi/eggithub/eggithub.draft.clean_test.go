package eggithub

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStaleDrafts(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

	t.Run("only drafts older than the ttl are returned", func(t *testing.T) {
		listing := `[
			{"tagName":"old-draft","isDraft":true,"createdAt":"2026-10-30T12:00:00Z"},
			{"tagName":"fresh-draft","isDraft":true,"createdAt":"2026-11-02T06:00:00Z"},
			{"tagName":"old-published","isDraft":false,"createdAt":"2026-10-01T12:00:00Z"}
		]`

		tags, err := staleDrafts(listing, now, defaultDraftTTL)
		require.NoError(t, err)
		require.Equal(t, []string{"old-draft"}, tags)
	})

	t.Run("the ttl decides how old a draft has to be", func(t *testing.T) {
		listing := `[{"tagName":"draft","isDraft":true,"createdAt":"2026-11-02T06:00:00Z"}]`

		tags, err := staleDrafts(listing, now, time.Hour)
		require.NoError(t, err)
		require.Equal(t, []string{"draft"}, tags)
	})

	t.Run("an empty listing has no stale drafts", func(t *testing.T) {
		tags, err := staleDrafts("[]", now, defaultDraftTTL)
		require.NoError(t, err)
		require.Empty(t, tags)
	})

	t.Run("invalid json is an error", func(t *testing.T) {
		_, err := staleDrafts("not json", now, defaultDraftTTL)
		require.Error(t, err)
	})
}
