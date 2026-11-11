package cmderrors_test

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"testing"

	"github.com/egdaemon/eg/cmd/cmderrors"
	"github.com/stretchr/testify/require"
)

type notification struct{ msg string }

func (n notification) Error() string { return n.msg }
func (notification) Notification()   {}

type friendly struct{ msg string }

func (f friendly) Error() string { return f.msg }
func (friendly) UserFriendly()   {}

type both struct{ msg string }

func (b both) Error() string { return b.msg }
func (both) Notification()   {}
func (both) UserFriendly()   {}

func TestSprint(t *testing.T) {
	t.Run("notification is printed verbatim", func(t *testing.T) {
		require.Equal(t, "hello", cmderrors.Sprint(notification{msg: "hello"}))
	})

	t.Run("wrapped notification is printed verbatim", func(t *testing.T) {
		err := fmt.Errorf("wrapped: %w", notification{msg: "hello"})
		require.Equal(t, "wrapped: hello", cmderrors.Sprint(err))
	})

	t.Run("user friendly is prefixed with ERROR", func(t *testing.T) {
		s := cmderrors.Sprint(friendly{msg: "bad input"})
		require.Contains(t, s, "ERROR")
		require.Contains(t, s, "bad input")
	})

	t.Run("wrapped user friendly is prefixed with ERROR", func(t *testing.T) {
		s := cmderrors.Sprint(fmt.Errorf("wrapped: %w", friendly{msg: "bad input"}))
		require.Contains(t, s, "ERROR")
		require.Contains(t, s, "wrapped: bad input")
	})

	t.Run("notification takes precedence over user friendly", func(t *testing.T) {
		require.Equal(t, "both", cmderrors.Sprint(both{msg: "both"}))
	})

	t.Run("unknown errors include type and detail", func(t *testing.T) {
		err := errors.New("boom")
		require.Equal(t, "*errors.errorString - [boom]", cmderrors.Sprint(err))
	})
}

func TestLogCause(t *testing.T) {
	t.Run("nil returns nil and logs nothing", func(t *testing.T) {
		var buf bytes.Buffer
		prev := log.Writer()
		log.SetOutput(&buf)
		t.Cleanup(func() { log.SetOutput(prev) })

		require.NoError(t, cmderrors.LogCause(nil))
		require.Empty(t, buf.String())
	})

	t.Run("notification is logged verbatim", func(t *testing.T) {
		var buf bytes.Buffer
		prev, flags := log.Writer(), log.Flags()
		log.SetOutput(&buf)
		log.SetFlags(0)
		t.Cleanup(func() { log.SetOutput(prev); log.SetFlags(flags) })

		err := notification{msg: "hello"}
		require.Equal(t, err, cmderrors.LogCause(err))
		require.Equal(t, "hello\n", buf.String())
	})

	t.Run("user friendly is logged with ERROR", func(t *testing.T) {
		var buf bytes.Buffer
		prev, flags := log.Writer(), log.Flags()
		log.SetOutput(&buf)
		log.SetFlags(0)
		t.Cleanup(func() { log.SetOutput(prev); log.SetFlags(flags) })

		err := friendly{msg: "bad input"}
		require.Equal(t, err, cmderrors.LogCause(err))
		require.Contains(t, buf.String(), "ERROR")
		require.Contains(t, buf.String(), "bad input")
	})

	t.Run("unknown errors are logged with type", func(t *testing.T) {
		var buf bytes.Buffer
		prev, flags := log.Writer(), log.Flags()
		log.SetOutput(&buf)
		log.SetFlags(0)
		t.Cleanup(func() { log.SetOutput(prev); log.SetFlags(flags) })

		err := errors.New("boom")
		require.Equal(t, err, cmderrors.LogCause(err))
		require.Equal(t, "*errors.errorString - [boom]\n", buf.String())
	})
}
