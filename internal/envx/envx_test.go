package envx_test

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/bytesx"
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

func TestBytes(t *testing.T) {
	const key = "a1e2f3c4-b5d6-7890-abce-012345678902"
	t.Setenv(key, strconv.FormatUint(6*bytesx.GiB, 10))
	require.Equal(t, uint64(6*bytesx.GiB), envx.Bytes(0, key))

	t.Setenv(key, "6GiB")
	require.Equal(t, uint64(6*bytesx.GiB), envx.Bytes(0, key))

	t.Setenv(key, "512MiB")
	require.Equal(t, uint64(512*bytesx.MiB), envx.Bytes(0, key))

	require.Equal(t, uint64(1), envx.Bytes(1, "missing-key-bytes"))

	// invalid value falls back
	t.Setenv(key, "notabytesize")
	require.Equal(t, uint64(7), envx.Bytes(7, key))
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

func TestBase64FallsBackToURL(t *testing.T) {
	const key = "c9d0e1f2-a3b4-5678-cdef-789012345678"
	raw := []byte{0xfb, 0xff, 0xfe, 0xfb, 0xff, 0xfe}

	// the url alphabet differs from the standard alphabet, only for these bytes.
	require.NotEqual(t, base64.StdEncoding.EncodeToString(raw), base64.URLEncoding.EncodeToString(raw))

	t.Setenv(key, base64.URLEncoding.EncodeToString(raw))
	require.Equal(t, raw, envx.Base64(nil, key))

	t.Setenv(key, "not base64!")
	require.Equal(t, []byte{0xff}, envx.Base64([]byte{0xff}, key))
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

func TestFormat(t *testing.T) {
	type named string
	instant := time.Date(2026, 11, 11, 13, 4, 5, 123456789, time.FixedZone("x", 2*60*60))

	for _, c := range []struct {
		name     string
		actual   string
		expected string
	}{
		{name: "string", actual: envx.Format("K", "value"), expected: "K=value"},
		{name: "named string", actual: envx.Format("K", named("value")), expected: "K=value"},
		{name: "empty string is ignored", actual: envx.Format("K", ""), expected: ""},
		{name: "empty string allowed", actual: envx.Format("K", "", envx.FormatOptionTransforms(func(s string) string { return s })), expected: "K="},
		{name: "true", actual: envx.Format("ENABLED", true), expected: "ENABLED=true"},
		{name: "false", actual: envx.Format("ENABLED", false), expected: "ENABLED=false"},
		{name: "int", actual: envx.Format("COUNT", 42), expected: "COUNT=42"},
		{name: "negative int", actual: envx.Format("COUNT", -1), expected: "COUNT=-1"},
		{name: "uint64", actual: envx.Format("COUNT", uint64(math.MaxUint64)), expected: "COUNT=18446744073709551615"},
		{name: "int8", actual: envx.Format("COUNT", int8(-8)), expected: "COUNT=-8"},
		{name: "float64", actual: envx.Format("RATIO", 0.25), expected: "RATIO=0.25"},
		{name: "float32", actual: envx.Format("RATIO", float32(1.5)), expected: "RATIO=1.5"},
		{name: "duration", actual: envx.Format("TTL", 90*time.Minute), expected: "TTL=1h30m0s"},
		{name: "zero duration", actual: envx.Format("TTL", time.Duration(0)), expected: "TTL=0s"},
		{name: "time is utc rfc3339nano", actual: envx.Format("AT", instant), expected: "AT=2026-11-11T11:04:05.123456789Z"},
		{name: "bytes are url base64", actual: envx.Format("KEY", []byte{0xfb, 0xff, 0xfe, 0x01}), expected: "KEY=-__-AQ=="},
		{name: "empty bytes are ignored", actual: envx.Format("KEY", []byte{}), expected: ""},
		{name: "zero time is ignored", actual: envx.Format("AT", time.Time{}), expected: ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.expected, c.actual)
		})
	}

	t.Run("omit zero", func(t *testing.T) {
		for _, c := range []struct {
			name     string
			actual   string
			expected string
		}{
			{name: "empty string", actual: envx.Format("K", "", envx.FormatOmitZero), expected: ""},
			{name: "string", actual: envx.Format("K", "v", envx.FormatOmitZero), expected: "K=v"},
			{name: "zero int", actual: envx.Format("K", 0, envx.FormatOmitZero), expected: ""},
			{name: "int", actual: envx.Format("K", 3, envx.FormatOmitZero), expected: "K=3"},
			{name: "false", actual: envx.Format("K", false, envx.FormatOmitZero), expected: ""},
			{name: "true", actual: envx.Format("K", true, envx.FormatOmitZero), expected: "K=true"},
			{name: "zero float", actual: envx.Format("K", 0.0, envx.FormatOmitZero), expected: ""},
			{name: "zero duration", actual: envx.Format("K", time.Duration(0), envx.FormatOmitZero), expected: ""},
			{name: "duration", actual: envx.Format("K", time.Second, envx.FormatOmitZero), expected: "K=1s"},
			{name: "zero time", actual: envx.Format("K", time.Time{}, envx.FormatOmitZero), expected: ""},
			{name: "empty bytes", actual: envx.Format("K", []byte{}, envx.FormatOmitZero), expected: ""},
			{name: "bytes", actual: envx.Format("K", []byte{1}, envx.FormatOmitZero), expected: "K=AQ=="},
			{name: "zero is kept without the option", actual: envx.Format("K", 0), expected: "K=0"},
		} {
			t.Run(c.name, func(t *testing.T) {
				require.Equal(t, c.expected, c.actual)
			})
		}

		env, err := envx.Build().
			Var("TTL", time.Duration(0), envx.FormatOmitZero).
			Var("ON", false, envx.FormatOmitZero).
			Var("KEPT", 0).
			Var("SET", time.Minute, envx.FormatOmitZero).
			Environ()
		require.NoError(t, err)
		require.Equal(t, []string{"KEPT=0", "SET=1m0s"}, env)
	})

	t.Run("quote", func(t *testing.T) {
		for _, c := range []struct {
			name     string
			actual   string
			expected string
		}{
			{name: "whitespace", actual: envx.Format("K", "100 GiB", envx.FormatQuote), expected: `K="100 GiB"`},
			{name: "plain", actual: envx.Format("K", "v", envx.FormatQuote), expected: `K="v"`},
			{name: "already quoted", actual: envx.Format("K", `"v"`, envx.FormatQuote), expected: `K="v"`},
			{name: "escapes", actual: envx.Format("K", `a"b\c`, envx.FormatQuote), expected: `K="a\"b\\c"`},
			{name: "number", actual: envx.Format("K", 3, envx.FormatQuote), expected: `K="3"`},
			{name: "duration", actual: envx.Format("K", time.Second, envx.FormatQuote), expected: `K="1s"`},
			{name: "empty is still ignored", actual: envx.Format("K", "", envx.FormatQuote), expected: ""},
			{name: "omit zero", actual: envx.Format("K", 0, envx.FormatQuote, envx.FormatOmitZero), expected: ""},
		} {
			t.Run(c.name, func(t *testing.T) {
				require.Equal(t, c.expected, c.actual)
			})
		}

		env, err := envx.Build().
			Var("A", "100 GiB", envx.FormatQuote).
			Var("B", "100 GiB").
			Var("C", "", envx.FormatQuote).
			Var("D", "", envx.FormatAllowEmpty).
			Environ()
		require.NoError(t, err)
		require.Equal(t, []string{`A="100 GiB"`, "B=100 GiB", "D="}, env)
	})

	t.Run("duration round trips", func(t *testing.T) {
		t.Setenv("0b1c2d3e-ttl", "48h0m0s")
		t.Setenv("0b1c2d3e-bytes", strings.TrimPrefix(envx.Format("0b1c2d3e-bytes", []byte{0xfb, 0xff, 0xfe, 0x01}), "0b1c2d3e-bytes="))
		require.Equal(t, []byte{0xfb, 0xff, 0xfe, 0x01}, envx.Base64(nil, "0b1c2d3e-bytes"))
		require.Equal(t, "0b1c2d3e-ttl=48h0m0s", envx.Format("0b1c2d3e-ttl", 48*time.Hour))
		require.Equal(t, 48*time.Hour, envx.Duration(0, "0b1c2d3e-ttl"))
	})
}

type marshaler struct{}

func (marshaler) MarshalText() ([]byte, error) { return []byte("100 GiB"), nil }

type stringer struct{}

func (stringer) String() string { return "stringer-value" }

func TestBuilderVarValues(t *testing.T) {
	instant := time.Date(2026, 11, 11, 13, 4, 5, 0, time.UTC)

	env, err := envx.Build().
		Var("S", "value").
		Var("B", true).
		Var("I", 42).
		Var("U", uint16(7)).
		Var("F", 0.5).
		Var("D", 90*time.Minute).
		Var("T", instant).
		Var("X", stringer{}).
		Var("M", marshaler{}).
		Var("QUOTED", "\"48h0m0s\"").
		Var("EMPTY", "").
		Var("ZERO", time.Time{}).
		Environ()
	require.NoError(t, err)
	require.Equal(t, []string{
		"S=value",
		"B=true",
		"I=42",
		"U=7",
		"F=0.5",
		"D=1h30m0s",
		"T=2026-11-11T13:04:05Z",
		"X=stringer-value",
		"M=100 GiB",
		"QUOTED=\"48h0m0s\"",
	}, env)

	allowed, err := envx.Build().
		Var("EMPTY", "", envx.FormatOptionTransforms(func(s string) string { return s })).
		Var("IGNORED", "").
		Environ()
	require.NoError(t, err)
	require.Equal(t, []string{"EMPTY="}, allowed)

	typed, err := envx.Var(envx.Var(envx.Build(), "N", 3), "D", time.Second).Environ()
	require.NoError(t, err)
	require.Equal(t, []string{"N=3", "D=1s"}, typed)
}

func TestVarsAndReaderSkipEmpty(t *testing.T) {
	require.Empty(t, envx.Vars("", "KEY1", "KEY2"))
	require.Equal(t, []string{"KEY1=true", "KEY2=true"}, envx.Vars(true, "KEY1", "KEY2"))
	require.Equal(t, []string{"KEY1=1h30m0s", "KEY2=1h30m0s"}, envx.Vars(90*time.Minute, "KEY1", "KEY2"))

	environ, err := envx.FromReader(strings.NewReader("A=1\nEMPTY=\nB=2\n"))
	require.NoError(t, err)
	require.Equal(t, []string{"A=1", "B=2"}, environ)
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

func TestSliceFormat(t *testing.T) {
	for _, c := range []struct {
		name     string
		actual   fmt.Stringer
		expected string
	}{
		{name: "strings", actual: envx.SliceFormat(",", "a", "b", "c"), expected: "a,b,c"},
		{name: "single", actual: envx.SliceFormat(",", "a"), expected: "a"},
		{name: "none", actual: envx.SliceFormat[string](","), expected: ""},
		{name: "empty values are dropped", actual: envx.SliceFormat(",", "a", "", "b"), expected: "a,b"},
		{name: "ints", actual: envx.SliceFormat(":", 1, 2, 3), expected: "1:2:3"},
		{name: "bools", actual: envx.SliceFormat(" ", true, false), expected: "true false"},
		{name: "durations", actual: envx.SliceFormat(",", time.Second, 90*time.Minute), expected: "1s,1h30m0s"},
	} {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.expected, c.actual.String())
		})
	}

	env, err := envx.Build().Var("LABELS", envx.SliceFormat(",", "a", "b")).Var("NONE", envx.SliceFormat[string](",")).Environ()
	require.NoError(t, err)
	require.Equal(t, []string{"LABELS=a,b"}, env)
}
