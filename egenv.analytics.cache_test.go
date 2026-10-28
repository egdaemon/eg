package eg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnalyticsCachePath(t *testing.T) {
	require.Equal(t, ".eg/main.wasm/latest.db", AnalyticsCachePath("main.wasm"))
	require.Equal(t, ".eg/default/latest.db", AnalyticsCachePath(""))
	require.Equal(t, ".eg/etc/passwd/latest.db", AnalyticsCachePath("../../etc/passwd"))
}
