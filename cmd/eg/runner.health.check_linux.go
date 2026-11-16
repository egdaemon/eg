//go:build linux

package main

import (
	"context"
	"log"
	"os/exec"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
)

// ensures the system is actual able to execute modules.
func systemReady(_ctx context.Context) error {
	var (
		e0 = errorsx.String("container systemd status is degraded or failed - this implies there will be issues with the runtime. enable debugging (-vv) for details.")
	)

	_, err := errorsx.IgnoreN[string](_ctx, 8, e0)(func(ctx context.Context) (string, error) {
		units := execx.Args(
			"reset-failed", "--wait",
			"sys-kernel-config.mount",
			"sys-kernel-debug.mount",
			"sys-kernel-tracing.mount",
			"systemd-journald-dev-log.socket",
			"systemd-journald.socket",
			"proc-sys-fs-binfmt_misc.automount",
			"ssh.socket",
			"tmp.mount",                         // mounting a tmpfs needs CAP_SYS_ADMIN, the container manager provides /tmp already.
			"dev-mqueue.mount",                  // same as tmp.mount, the container manager mounts /dev/mqueue itself.
			"systemd-ask-password-console.path", // nothing prompts for passwords in a headless container.
			"systemd-ask-password-wall.path",    // same as the console path.
			"systemd-resolved.service",          // sandboxing needs mount namespaces, fails with 226/NAMESPACE when unprivileged. the container manager owns /etc/resolv.conf.
			"systemd-resolved-monitor.socket",   // belongs to systemd-resolved.service, fails with it.
			"systemd-resolved-varlink.socket",   // belongs to systemd-resolved.service, fails with it.
		).Append(envx.List(envx.New(""), eg.EnvComputeSystemdFailedReset)...)
		// system predefined services we have deemed reasonable to ignore, and those the environment adds at runtime.
		errorsx.ZeroDebug[string](debugx.Fn)(execx.String(ctx, "systemctl", units...))

		o, err := execx.String(ctx, "systemctl", "is-system-running", "--wait")
		debugx.Fn(func() {
			log.Println("units reset", units)
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
