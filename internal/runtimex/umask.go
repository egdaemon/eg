//go:build linux || darwin

package runtimex

import (
	"log"
	"syscall"
)

func Umask(m int) int {
	newm := syscall.Umask(m)
	log.Println("umask set to", newm)
	return newm
}
