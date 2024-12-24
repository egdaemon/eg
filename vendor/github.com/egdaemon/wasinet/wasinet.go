package wasinet

import (
	"net"
)

const (
	Namespace = "wasinet_v0"
)

// hijack the net.DefaultResolver
func Hijack() {
	net.DefaultResolver.Dial = DialContext
}
