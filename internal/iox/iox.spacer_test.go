package iox_test

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/egdaemon/eg/internal/iox"
	"github.com/stretchr/testify/require"
)

func TestSpacerFrom(t *testing.T) {
	t.Run("a previous byte that is not a newline adds one", func(t *testing.T) {
		got, err := io.ReadAll(iox.SpacerFrom([]byte("1"), strings.NewReader("B=2\n")))
		require.NoError(t, err)
		require.Equal(t, "\nB=2\n", string(got))
	})

	t.Run("a previous newline adds nothing", func(t *testing.T) {
		got, err := io.ReadAll(iox.SpacerFrom([]byte("\n"), strings.NewReader("B=2\n")))
		require.NoError(t, err)
		require.Equal(t, "B=2\n", string(got))
	})

	t.Run("nothing before adds nothing", func(t *testing.T) {
		got, err := io.ReadAll(iox.SpacerFrom(nil, strings.NewReader("B=2\n")))
		require.NoError(t, err)
		require.Equal(t, "B=2\n", string(got))
	})

	t.Run("a previous nul byte is data and adds a newline", func(t *testing.T) {
		got, err := io.ReadAll(iox.SpacerFrom([]byte{0}, strings.NewReader("B=2\n")))
		require.NoError(t, err)
		require.Equal(t, "\nB=2\n", string(got))
	})

	t.Run("only the end of the previous bytes matters", func(t *testing.T) {
		got, err := io.ReadAll(iox.SpacerFrom([]byte("A=1\n"), strings.NewReader("B=2\n")))
		require.NoError(t, err)
		require.Equal(t, "B=2\n", string(got))
	})

	t.Run("a single byte is enough", func(t *testing.T) {
		got, err := io.ReadAll(iox.SpacerFrom([]byte("1"), strings.NewReader("x")))
		require.NoError(t, err)
		require.Equal(t, "\nx", string(got))
	})

	t.Run("an empty reader adds nothing", func(t *testing.T) {
		got, err := io.ReadAll(iox.SpacerFrom([]byte("1"), strings.NewReader("")))
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("an error is returned", func(t *testing.T) {
		cause := errors.New("boom")
		got, err := io.ReadAll(iox.SpacerFrom([]byte("1"), iotest.ErrReader(cause)))
		require.ErrorIs(t, err, cause)
		require.Empty(t, got)
	})

	t.Run("only one byte is read from the source up front", func(t *testing.T) {
		src := strings.NewReader("abc")
		_ = iox.SpacerFrom([]byte("1"), src)
		require.Equal(t, 2, src.Len())
	})
}

func TestTail(t *testing.T) {
	t.Run("last is empty before anything is written", func(t *testing.T) {
		tail := iox.NewTail(1)
		require.Empty(t, tail.Last())
	})

	t.Run("last is the final bytes up to the size", func(t *testing.T) {
		tail := iox.NewTail(3)
		_, err := tail.Write([]byte("A=1"))
		require.NoError(t, err)
		_, err = tail.Write([]byte("B=2\n"))
		require.NoError(t, err)
		require.Equal(t, "=2\n", string(tail.Last()))
	})

	t.Run("fewer bytes than the size are all kept", func(t *testing.T) {
		tail := iox.NewTail(8)
		_, err := tail.Write([]byte("ab"))
		require.NoError(t, err)
		require.Equal(t, "ab", string(tail.Last()))
	})

	t.Run("an empty write keeps the last bytes", func(t *testing.T) {
		tail := iox.NewTail(1)
		_, err := tail.Write([]byte("A=1"))
		require.NoError(t, err)
		_, err = tail.Write(nil)
		require.NoError(t, err)
		require.Equal(t, "1", string(tail.Last()))
	})

	t.Run("a nul byte is remembered", func(t *testing.T) {
		tail := iox.NewTail(1)
		_, err := tail.Write([]byte{'a', 0})
		require.NoError(t, err)
		require.Equal(t, []byte{0}, tail.Last())
	})
}
