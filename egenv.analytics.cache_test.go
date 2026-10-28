package eg

import (
	"testing"

	"github.com/egdaemon/eg/internal/md5x"
	"github.com/stretchr/testify/require"
)

func TestAnalyticsCachePath(t *testing.T) {
	require.Equal(t, ".eg/db/"+md5x.String("foo")+"/latest.db", AnalyticsCachePath("foo"))
	require.Equal(t, ".eg/db/"+md5x.String(DefaultWorkload)+"/latest.db", AnalyticsCachePath(""))
	require.Equal(t, AnalyticsCachePath(""), AnalyticsCachePath("  "))
	require.NotEqual(t, AnalyticsCachePath("a"), AnalyticsCachePath("b"))

	// any name is a single, safe directory.
	require.Equal(t, ".eg/db/"+md5x.String("../../etc/passwd")+"/latest.db", AnalyticsCachePath("../../etc/passwd"))
}
