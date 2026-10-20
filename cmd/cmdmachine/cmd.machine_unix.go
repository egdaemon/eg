//go:build !windows

package cmdmachine

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

// watchresize forwards terminal resizes to the session until the returned function is called.
func watchresize(ctx context.Context, fd int, session *ssh.Session) func() {
	ctx, done := context.WithCancel(ctx)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGWINCH)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigs:
				if w, h, err := term.GetSize(fd); err == nil {
					_ = session.WindowChange(h, w)
				}
			}
		}
	}()

	return func() {
		signal.Stop(sigs)
		done()
	}
}
