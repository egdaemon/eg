//go:build linux

package main

import (
	"context"
	"log"
	"os/exec"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
)

// ensures the system is actual able to execute modules.
func systemReady(_ctx context.Context) error {
	var (
		e0 = errorsx.String("container systemd status is degraded or failed - this implies there will be issues with the runtime")
	)

	_, err := errorsx.IgnoreN[string](_ctx, 8, e0)(func(ctx context.Context) (string, error) {
		errorsx.Zero(execx.String(ctx, "systemctl", "reset-failed", "--wait", "sys-kernel-config.mount", "sys-kernel-debug.mount", "sys-kernel-tracing.mount", "systemd-journald-dev-log.socket", "systemd-journald.socket"))
		o, err := execx.String(ctx, "systemctl", "is-system-running", "--wait")
		debugx.Fn(func() {
			log.Println("systemctl is-system-running --wait")
			log.Println("curl --unix-socket /run/podman/podman.sock http://localhost/_ping")
		})
		switch err.(type) {
		case *exec.ExitError:
			return o, e0
		default:
		}

		o, err = execx.String(ctx, "curl", "--unix-socket", "/run/podman/podman.sock", "http://localhost/_ping")
		switch err.(type) {
		case *exec.ExitError:
			return o, e0
		default:
			return o, err
		}
	})

	if err != nil {
		debugx.Fn(func() {
			log.Println(errorsx.Zero(execx.String(_ctx, "systemctl", "list-jobs")))
			log.Println(errorsx.Zero(execx.String(_ctx, "systemctl", "status")))
			log.Println(errorsx.Zero(execx.String(_ctx, "systemctl", "list-units", "--failed")))
		})
		return errorsx.Wrap(err, "container health check failed")
	}

	return nil
}
