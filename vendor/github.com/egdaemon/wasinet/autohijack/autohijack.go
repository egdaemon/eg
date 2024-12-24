//go:build wasip1

// Package autohijack automatically highjack the net.DefaultResolver
package autohijack

import "github.com/egdaemon/wasinet"

func init() {
	wasinet.Hijack()
}
