package envx_test

import (
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/stretchr/testify/require"
)

func TestToggle(t *testing.T) {
	require.Equal(t, "on", envx.Toggle("off", "on", true))
	require.Equal(t, "off", envx.Toggle("off", "on", false))
}

func TestNewEnvironFromStrings(t *testing.T) {
	e := envx.NewEnvironFromStrings("a1b2c3d4=hello", "e5f67890=world", "malformed")
	require.Equal(t, "hello", e.Map("a1b2c3d4"))
	require.Equal(t, "world", e.Map("e5f67890"))
	require.Equal(t, "", e.Map("missing"))
}

func TestNewEnviron(t *testing.T) {
	e := envx.NewEnviron(func(s string) string {
		if s == "3f7c8b2a-1234-5678-9abc-def012345678" {
			return "found"
		}
		return ""
	})
	require.Equal(t, "found", e.Map("3f7c8b2a-1234-5678-9abc-def012345678"))
	require.Equal(t, "", e.Map("other"))
}

func TestInt(t *testing.T) {
	const key = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	t.Setenv(key, "42")
	require.Equal(t, 42, envx.Int(0, key))
	require.Equal(t, 99, envx.Int(99, "missing-key-int"))
	// invalid value falls back
	t.Setenv(key, "notanint")
	require.Equal(t, 7, envx.Int(7, key))
}

func TestUint64(t *testing.T) {
	const key = "b2c3d4e5-f6a7-8901-bcde-f12345678901"
	t.Setenv(key, "18446744073709551615")
	require.Equal(t, uint64(18446744073709551615), envx.Uint64(0, key))
	require.Equal(t, uint64(1), envx.Uint64(1, "missing-key-uint64"))
}

func TestFloat64(t *testing.T) {
	const key = "c3d4e5f6-a7b8-9012-cdef-123456789012"
	t.Setenv(key, "3.14")
	require.InDelta(t, 3.14, envx.Float64(0, key), 0.0001)
	require.InDelta(t, 1.0, envx.Float64(1.0, "missing-key-float64"), 0.0001)
}

func TestBoolean(t *testing.T) {
	const key = "d4e5f6a7-b8c9-0123-defa-234567890123"
	t.Setenv(key, "true")
	require.True(t, envx.Boolean(false, key))
	t.Setenv(key, "false")
	require.False(t, envx.Boolean(true, key))
	require.True(t, envx.Boolean(true, "missing-key-bool"))
}

func TestString(t *testing.T) {
	const key = "e5f6a7b8-c9d0-1234-efab-345678901234"
	t.Setenv(key, "e5f6a7b8-c9d0-1234-efab-345678901234")
	require.Equal(t, "e5f6a7b8-c9d0-1234-efab-345678901234", envx.String("fallback", key))
	require.Equal(t, "fallback", envx.String("fallback", "missing-key-string"))
}

func TestDuration(t *testing.T) {
	const key = "f6a7b8c9-d0e1-2345-fabc-456789012345"
	t.Setenv(key, "5m30s")
	require.Equal(t, 5*time.Minute+30*time.Second, envx.Duration(0, key))
	require.Equal(t, time.Second, envx.Duration(time.Second, "missing-key-duration"))
}

func TestHex(t *testing.T) {
	const key = "a7b8c9d0-e1f2-3456-abcd-567890123456"
	raw := []byte{0xde, 0xad, 0xbe, 0xef}
	t.Setenv(key, hex.EncodeToString(raw))
	require.Equal(t, raw, envx.Hex(nil, key))
	require.Equal(t, []byte{0xff}, envx.Hex([]byte{0xff}, "missing-key-hex"))
}

func TestBase64(t *testing.T) {
	const key = "b8c9d0e1-f2a3-4567-bcde-678901234567"
	raw := []byte("b8c9d0e1-f2a3-4567-bcde-678901234567")
	t.Setenv(key, base64.StdEncoding.EncodeToString(raw))
	require.Equal(t, raw, envx.Base64(nil, key))
	require.Equal(t, []byte{0xff}, envx.Base64([]byte{0xff}, "missing-key-base64"))
}

func TestURL(t *testing.T) {
	const key = "c9d0e1f2-a3b4-5678-cdef-789012345678"
	t.Setenv(key, "https://example.com/path")
	u := envx.URL("https://fallback.example.com", key)
	require.Equal(t, "https", u.Scheme)
	require.Equal(t, "example.com", u.Host)
	require.Equal(t, "/path", u.Path)

	fallback := envx.URL("https://fallback.example.com", "missing-key-url")
	require.Equal(t, "fallback.example.com", fallback.Host)
}

func TestEnvironMethods(t *testing.T) {
	e := envx.NewEnvironFromStrings(
		"d0e1f2a3-b4c5-6789-defa-890123456789=42",
		"e1f2a3b4-c5d6-7890-efab-901234567890=3.14",
		"f2a3b4c5-d6e7-8901-fabc-012345678901=true",
		"a3b4c5d6-e7f8-9012-abcd-123456789012=5m",
		"b4c5d6e7-f8a9-0123-bcde-234567890123=deadbeef",
	)

	require.Equal(t, 42, e.Int(0, "d0e1f2a3-b4c5-6789-defa-890123456789"))
	require.InDelta(t, 3.14, e.Float64(0, "e1f2a3b4-c5d6-7890-efab-901234567890"), 0.0001)
	require.True(t, e.Boolean(false, "f2a3b4c5-d6e7-8901-fabc-012345678901"))
	require.Equal(t, 5*time.Minute, e.Duration(0, "a3b4c5d6-e7f8-9012-abcd-123456789012"))
	require.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, e.Hex(nil, "b4c5d6e7-f8a9-0123-bcde-234567890123"))
}

func TestBuilderVar(t *testing.T) {
	b := envx.Build().Var("c5d6e7f8-a9b0-1234-cdef-345678901234", "c5d6e7f8-a9b0-1234-cdef-345678901234")
	env, err := b.Environ()
	require.NoError(t, err)
	require.Contains(t, env, "c5d6e7f8-a9b0-1234-cdef-345678901234=c5d6e7f8-a9b0-1234-cdef-345678901234")
}

func TestBuilderFromEnv(t *testing.T) {
	const key = "d6e7f8a9-b0c1-2345-defa-456789012345"
	t.Setenv(key, "d6e7f8a9-b0c1-2345-defa-456789012345")

	b := envx.Build().FromEnv(key, "missing-key")
	env, err := b.Environ()
	require.NoError(t, err)
	require.Contains(t, env, key+"=d6e7f8a9-b0c1-2345-defa-456789012345")
	require.Len(t, env, 1) // missing-key not present
}

func TestVars(t *testing.T) {
	result := envx.Vars("e7f8a9b0-c1d2-3456-efab-567890123456", "KEY1", "KEY2")
	require.Len(t, result, 2)
	require.Contains(t, result, "KEY1=e7f8a9b0-c1d2-3456-efab-567890123456")
	require.Contains(t, result, "KEY2=e7f8a9b0-c1d2-3456-efab-567890123456")
}

func TestVarBool(t *testing.T) {
	require.Equal(t, "true", envx.VarBool(true))
	require.Equal(t, "false", envx.VarBool(false))
}

func TestFormatBool(t *testing.T) {
	require.Equal(t, "ENABLED=true", envx.FormatBool("ENABLED", true))
	require.Equal(t, "ENABLED=false", envx.FormatBool("ENABLED", false))
}

func TestFormatInt(t *testing.T) {
	require.Equal(t, "COUNT=42", envx.FormatInt("COUNT", 42))
	require.Equal(t, "COUNT=-1", envx.FormatInt("COUNT", -1))
}

func TestFormatOptionTransforms(t *testing.T) {
	noop := func(s string) string { return s }
	opt := envx.FormatOptionTransforms(noop)
	result := envx.Format("KEY", "value", opt)
	require.Equal(t, "KEY=value", result)
}

func TestDirty(t *testing.T) {
	require.NotEmpty(t, envx.Dirty(true))
	require.Empty(t, envx.Dirty(false))
}

func TestFromPath(t *testing.T) {
	t.Run("reads_vars_from_file", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "env.txt")
		require.NoError(t, os.WriteFile(p, []byte("KEY1=val1\nKEY2=val2\n"), 0600))
		env, err := envx.FromPath(p)
		require.NoError(t, err)
		require.Equal(t, []string{"KEY1=val1", "KEY2=val2"}, env)
	})

	t.Run("permission_denied_returns_error", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "unreadable.*")
		require.NoError(t, err)
		f.Close()
		require.NoError(t, os.Chmod(f.Name(), 0000))

		_, err = envx.FromPath(f.Name())
		require.ErrorIs(t, err, os.ErrPermission)
	})
}

func TestCopyToWriteFailure(t *testing.T) {
	b := envx.Build().FromEnviron("KEY=val")
	require.ErrorIs(t, b.CopyTo(errorsx.Writer(os.ErrPermission)), os.ErrPermission)
}
