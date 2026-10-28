package egautogentest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseViolations(t *testing.T) {
	t.Run("no output means no violations", func(t *testing.T) {
		v, err := parseViolations("", 2)
		require.NoError(t, err)
		require.Empty(t, v)
	})

	t.Run("an empty array means no violations", func(t *testing.T) {
		v, err := parseViolations("[]\n", 2)
		require.NoError(t, err)
		require.Empty(t, v)
	})

	t.Run("one record", func(t *testing.T) {
		v, err := parseViolations(`{"rule": 1, "line": 12, "reason": "helper"}`, 2)
		require.NoError(t, err)
		require.Equal(t, []violation{{Rule: 1, Line: 12, Reason: "helper"}}, v)
	})

	t.Run("several records for one rule", func(t *testing.T) {
		v, err := parseViolations("{\"rule\": 2, \"line\": 3, \"reason\": \"a\"}\n\n{\"rule\": 2, \"line\": 9, \"reason\": \"b\"}\n", 2)
		require.NoError(t, err)
		require.Equal(t, []violation{{Rule: 2, Line: 3, Reason: "a"}, {Rule: 2, Line: 9, Reason: "b"}}, v)
	})

	t.Run("fenced response", func(t *testing.T) {
		_, err := parseViolations("```json\n{\"rule\": 1, \"line\": 1, \"reason\": \"x\"}\n```", 1)
		require.Error(t, err)
	})

	t.Run("undecodable record", func(t *testing.T) {
		_, err := parseViolations("the test looks fine", 1)
		require.Error(t, err)
	})

	t.Run("out of range rule", func(t *testing.T) {
		_, err := parseViolations(`{"rule": 3, "line": 1, "reason": "x"}`, 2)
		require.Error(t, err)

		_, err = parseViolations(`{"rule": 0, "line": 1, "reason": "x"}`, 2)
		require.Error(t, err)
	})
}
