package execproxy

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExec(t *testing.T) {
	dir := t.TempDir()

	t.Run("cancelled", func(t *testing.T) {
		t.Run("a command that handles SIGTERM ends before the delay", func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			proxy := NewExecProxy(t.Context(), dir, nil)

			done := make(chan error, 1)
			go func() {
				_, err := proxy.Exec(ctx, &ExecRequest{Cmd: "sh", Arguments: []string{"-c", "trap 'exit 0' TERM; while :; do sleep 0.05; done"}})
				done <- err
			}()

			time.Sleep(300 * time.Millisecond)
			cancel()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("the command was not ended by the cancel")
			}
		})

		t.Run("a command that ignores SIGTERM is killed after the delay", func(t *testing.T) {
			before := gracedelay
			gracedelay = 500 * time.Millisecond
			defer func() { gracedelay = before }()

			ctx, cancel := context.WithCancel(t.Context())
			proxy := NewExecProxy(t.Context(), dir, nil)

			done := make(chan error, 1)
			go func() {
				_, err := proxy.Exec(ctx, &ExecRequest{Cmd: "sh", Arguments: []string{"-c", "trap '' TERM; while :; do sleep 0.05; done"}})
				done <- err
			}()

			time.Sleep(300 * time.Millisecond)
			start := time.Now()
			cancel()

			select {
			case err := <-done:
				require.Error(t, err)
				require.GreaterOrEqual(t, time.Since(start), 400*time.Millisecond, "it was not given the delay")
			case <-time.After(5 * time.Second):
				t.Fatal("the command was not killed after the delay")
			}
		})
	})

	t.Run("the proxy is done", func(t *testing.T) {
		t.Run("a command ends although its request is not cancelled", func(t *testing.T) {
			proxyctx, done := context.WithCancel(t.Context())
			proxy := NewExecProxy(proxyctx, dir, nil)

			ended := make(chan error, 1)
			go func() {
				_, err := proxy.Exec(t.Context(), &ExecRequest{Cmd: "sh", Arguments: []string{"-c", "trap 'exit 0' TERM; while :; do sleep 0.05; done"}})
				ended <- err
			}()

			time.Sleep(300 * time.Millisecond)
			done()

			select {
			case <-ended:
			case <-time.After(5 * time.Second):
				t.Fatal("the command outlived the proxy's context")
			}
		})
	})
}
