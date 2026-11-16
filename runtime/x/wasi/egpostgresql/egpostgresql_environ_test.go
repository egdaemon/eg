package egpostgresql_test

import (
	"strings"
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egpostgresql"
	"github.com/stretchr/testify/require"
)

func TestEnviron(t *testing.T) {
	t.Setenv("EGPOSTGRESQL_TEST_INHERIT", "value")
	t.Setenv("GNUPGHOME", "/eg.mnt/.gnupg")

	environ := egpostgresql.Environ()
	require.Contains(t, environ, "PGHOST=localhost")
	require.True(t, strings.HasPrefix(environ[0], "PGPORT=") || strings.HasPrefix(environ[1], "PGPORT="), "expected PGPORT in %v", environ)
	require.NotContains(t, environ, "EGPOSTGRESQL_TEST_INHERIT=value")
	require.NotContains(t, environ, "GNUPGHOME=/eg.mnt/.gnupg")
	require.Len(t, environ, 2)
}
