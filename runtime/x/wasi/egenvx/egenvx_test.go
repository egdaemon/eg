package egenvx_test

import (
	"context"
	"testing"
	"time"

	"github.com/egdaemon/eg/runtime/x/wasi/egenvx"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		require.Equal(t, "default", egenvx.New("default").From("EG_TEST_EGENVX_STRING"))
		t.Setenv("EG_TEST_EGENVX_STRING", "value")
		require.Equal(t, "value", egenvx.New("default").From("EG_TEST_EGENVX_STRING"))
	})

	t.Run("unsigned integer keeps its type", func(t *testing.T) {
		var port uint = egenvx.New(uint(8080)).From("EG_TEST_EGENVX_PORT")
		require.Equal(t, uint(8080), port)
		t.Setenv("EG_TEST_EGENVX_PORT", "9090")
		port = egenvx.New(uint(8080)).From("EG_TEST_EGENVX_PORT")
		require.Equal(t, uint(9090), port)
	})

	t.Run("int, int64, uint64, float64 and bool", func(t *testing.T) {
		t.Setenv("EG_TEST_EGENVX_INT", "-3")
		t.Setenv("EG_TEST_EGENVX_INT64", "9007199254740993")
		t.Setenv("EG_TEST_EGENVX_UINT64", "18446744073709551615")
		t.Setenv("EG_TEST_EGENVX_FLOAT", "1.5")
		t.Setenv("EG_TEST_EGENVX_BOOL", "true")
		require.Equal(t, -3, egenvx.New(0).From("EG_TEST_EGENVX_INT"))
		require.Equal(t, int64(9007199254740993), egenvx.New(int64(0)).From("EG_TEST_EGENVX_INT64"))
		require.Equal(t, uint64(18446744073709551615), egenvx.New(uint64(0)).From("EG_TEST_EGENVX_UINT64"))
		require.Equal(t, 1.5, egenvx.New(0.0).From("EG_TEST_EGENVX_FLOAT"))
		require.True(t, egenvx.New(false).From("EG_TEST_EGENVX_BOOL"))
	})

	t.Run("defined types", func(t *testing.T) {
		type name string
		type count int
		t.Setenv("EG_TEST_EGENVX_NAME", "eg")
		t.Setenv("EG_TEST_EGENVX_COUNT", "4")
		require.Equal(t, name("eg"), egenvx.New(name("x")).From("EG_TEST_EGENVX_NAME"))
		require.Equal(t, count(4), egenvx.New(count(1)).From("EG_TEST_EGENVX_COUNT"))
	})

	t.Run("an invalid value falls back to the default", func(t *testing.T) {
		t.Setenv("EG_TEST_EGENVX_INVALID", "not a number")
		require.Equal(t, 7, egenvx.New(7).From("EG_TEST_EGENVX_INVALID"))
	})

	t.Run("an invalid value does not hide a later key", func(t *testing.T) {
		t.Setenv("EG_TEST_EGENVX_INVALID", "not a number")
		t.Setenv("EG_TEST_EGENVX_VALID", "9")
		require.Equal(t, 9, egenvx.New(7).From("EG_TEST_EGENVX_INVALID", "EG_TEST_EGENVX_VALID"))
	})

	t.Run("the first key that has a value wins", func(t *testing.T) {
		t.Setenv("EG_TEST_EGENVX_SECOND", "second")
		require.Equal(t, "second", egenvx.New("default").From("EG_TEST_EGENVX_FIRST", "EG_TEST_EGENVX_SECOND"))
		t.Setenv("EG_TEST_EGENVX_FIRST", "first")
		require.Equal(t, "first", egenvx.New("default").From("EG_TEST_EGENVX_FIRST", "EG_TEST_EGENVX_SECOND"))
	})

	t.Run("blank values are missing", func(t *testing.T) {
		t.Setenv("EG_TEST_EGENVX_BLANK", "  \t")
		require.Equal(t, "default", egenvx.New("default").From("EG_TEST_EGENVX_BLANK"))
	})

	t.Run("no keys is the default", func(t *testing.T) {
		require.Equal(t, "default", egenvx.New("default").From())
	})
}

func TestNewBytes(t *testing.T) {
	require.Equal(t, uint64(5), egenvx.NewBytes(5).From("EG_TEST_EGENVX_BYTES"))
	t.Setenv("EG_TEST_EGENVX_BYTES", "6GiB")
	require.Equal(t, uint64(6*1024*1024*1024), egenvx.NewBytes(5).From("EG_TEST_EGENVX_BYTES"))
	t.Setenv("EG_TEST_EGENVX_BYTES", "1024")
	require.Equal(t, uint64(1024), egenvx.NewBytes(5).From("EG_TEST_EGENVX_BYTES"))
	t.Setenv("EG_TEST_EGENVX_BYTES", "lots")
	require.Equal(t, uint64(5), egenvx.NewBytes(5).From("EG_TEST_EGENVX_BYTES"))
}

func TestNewDuration(t *testing.T) {
	require.Equal(t, time.Minute, egenvx.NewDuration(time.Minute).From("EG_TEST_EGENVX_DURATION"))
	t.Setenv("EG_TEST_EGENVX_DURATION", "90s")
	require.Equal(t, 90*time.Second, egenvx.NewDuration(time.Minute).From("EG_TEST_EGENVX_DURATION"))
	t.Setenv("EG_TEST_EGENVX_DURATION", "soon")
	require.Equal(t, time.Minute, egenvx.NewDuration(time.Minute).From("EG_TEST_EGENVX_DURATION"))
}

func TestNewHex(t *testing.T) {
	require.Equal(t, []byte{1}, egenvx.NewHex([]byte{1}).From("EG_TEST_EGENVX_HEX"))
	t.Setenv("EG_TEST_EGENVX_HEX", "deadbeef")
	require.Equal(t, []byte{0xde, 0xad, 0xbe, 0xef}, egenvx.NewHex([]byte{1}).From("EG_TEST_EGENVX_HEX"))
	t.Setenv("EG_TEST_EGENVX_HEX", "zz")
	require.Equal(t, []byte{1}, egenvx.NewHex([]byte{1}).From("EG_TEST_EGENVX_HEX"))
}

func TestNewBase64(t *testing.T) {
	require.Equal(t, []byte{1}, egenvx.NewBase64([]byte{1}).From("EG_TEST_EGENVX_BASE64"))
	t.Setenv("EG_TEST_EGENVX_BASE64", "aGVsbG8=")
	require.Equal(t, []byte("hello"), egenvx.NewBase64([]byte{1}).From("EG_TEST_EGENVX_BASE64"))
	t.Setenv("EG_TEST_EGENVX_BASE64", "***")
	require.Equal(t, []byte{1}, egenvx.NewBase64([]byte{1}).From("EG_TEST_EGENVX_BASE64"))
}

func TestNewURL(t *testing.T) {
	require.Equal(t, "https://example.com", egenvx.NewURL("https://example.com").From("EG_TEST_EGENVX_URL").String())
	t.Setenv("EG_TEST_EGENVX_URL", "https://eg.test/path")
	require.Equal(t, "https://eg.test/path", egenvx.NewURL("https://example.com").From("EG_TEST_EGENVX_URL").String())
	require.Panics(t, func() { egenvx.NewURL("://not a url") })
}

func TestExists(t *testing.T) {
	t.Run("no keys", func(t *testing.T) {
		require.False(t, egenvx.Exists())
	})

	t.Run("missing", func(t *testing.T) {
		require.False(t, egenvx.Exists("EG_TEST_EGENVX_MISSING"))
	})

	t.Run("empty and whitespace values are missing", func(t *testing.T) {
		t.Setenv("EG_TEST_EGENVX_EMPTY", "")
		t.Setenv("EG_TEST_EGENVX_BLANK", "  \t")
		require.False(t, egenvx.Exists("EG_TEST_EGENVX_EMPTY"))
		require.False(t, egenvx.Exists("EG_TEST_EGENVX_BLANK"))
	})

	t.Run("set", func(t *testing.T) {
		t.Setenv("EG_TEST_EGENVX_SET", "value")
		require.True(t, egenvx.Exists("EG_TEST_EGENVX_SET"))
	})

	t.Run("any key", func(t *testing.T) {
		t.Setenv("EG_TEST_EGENVX_ANY", "value")
		require.True(t, egenvx.Exists("EG_TEST_EGENVX_MISSING", "EG_TEST_EGENVX_ANY"))
		require.False(t, egenvx.Exists("EG_TEST_EGENVX_MISSING", "EG_TEST_EGENVX_ALSO_MISSING"))
	})
}

func TestDeferred(t *testing.T) {
	t.Run("reads the environment when called, not when created", func(t *testing.T) {
		exists := egenvx.Deferred(egenvx.Exists, "EG_TEST_EGENVX_TOKEN")
		require.False(t, exists(context.Background()))

		t.Setenv("EG_TEST_EGENVX_TOKEN", "value")
		require.True(t, exists(context.Background()))

		t.Setenv("EG_TEST_EGENVX_TOKEN", "")
		require.False(t, exists(context.Background()))
	})

	t.Run("composes with a reader", func(t *testing.T) {
		ttl := egenvx.Deferred(egenvx.NewDuration(time.Minute).From, "EG_TEST_EGENVX_TTL")
		require.Equal(t, time.Minute, ttl(context.Background()))

		t.Setenv("EG_TEST_EGENVX_TTL", "2h")
		require.Equal(t, 2*time.Hour, ttl(context.Background()))
	})

	t.Run("any key", func(t *testing.T) {
		t.Setenv("EG_TEST_EGENVX_SECOND", "second")
		exists := egenvx.Deferred(egenvx.Exists, "EG_TEST_EGENVX_FIRST", "EG_TEST_EGENVX_SECOND")
		require.True(t, exists(context.Background()))
	})
}
