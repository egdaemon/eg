//go:build !linux && !darwin

package runtimex

import (
	"log"
	"runtime"
)

func umask() {
}
func Umask(m int) int {
	log.Println("umask not set for", runtime.GOOS)
	return -1
}
