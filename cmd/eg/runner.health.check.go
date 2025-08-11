//go:build !linux

package main

import (
	"context"
)

// ensures the system is actual able to execute modules.
func systemReady(_ctx context.Context) error {
	return nil
}
