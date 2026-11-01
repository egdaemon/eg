package logx_test

import (
	"bytes"
	"log"
	"os"
	"testing"

	"github.com/egdaemon/eg/internal/logx"
	"github.com/stretchr/testify/require"
)

func TestIf(t *testing.T) {
	t.Run("logs when enabled", func(t *testing.T) {
		var out bytes.Buffer
		log.SetOutput(&out)
		defer log.SetOutput(os.Stderr)

		logx.If(true).Printf("hello %s\n", "world")
		require.Contains(t, out.String(), "hello world")
	})

	t.Run("drops the message when disabled", func(t *testing.T) {
		var out bytes.Buffer
		log.SetOutput(&out)
		defer log.SetOutput(os.Stderr)

		logx.If(false).Printf("hello %s\n", "world")
		require.Empty(t, out.String())
	})
}
