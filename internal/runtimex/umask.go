//go:build linux || darwin

package runtimex

import (
	"syscall"

	"github.com/egdaemon/eg/internal/debugx"
)

func Umask(m int) int {
	newm := syscall.Umask(m)
	debugx.Println("umask set to", newm)
	return newm
}
