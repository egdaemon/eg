//go:build windows

package cmdmachine

import (
	"context"

	"golang.org/x/crypto/ssh"
)

func watchresize(ctx context.Context, fd int, session *ssh.Session) func() {
	return func() {}
}
