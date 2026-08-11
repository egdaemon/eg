// Package wayland provides pipeline steps for bridging the host Wayland
// compositor into a container. AgentOptionWayland (in the eg CLI's runners
// package) always sets up the env vars and host-socket mount; newer eg CLI
// binaries also bridge the two automatically during container bootstrap
// (waylandhack, in cmd/eg/runner.go). Backport exists for pipelines that may
// run against an older eg CLI binary lacking that bootstrap step.
package wayland

import (
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egfs"
)

// Backport runs the waypipe client/server bridge as an ordinary pipeline
// step, but only if the container bootstrap hasn't already created the
// app-facing Wayland socket -- i.e. only on eg CLI versions old enough to
// lack waylandhack. Safe to include unconditionally in a pipeline.
func Backport() eg.OpFn {
	return eg.Sequential(
		eg.WhenFn(
			egfs.FileNotExistsFn(egenv.String("/tmp/wayland.sock", "WAYLAND_DISPLAY")),
			shell.Op(
				shell.Env().New(`sh -c '
					nohup waypipe -s "${WAYPIPE_CTRL_SOCKET}" client > /tmp/waypipe-client.log 2>&1 &
					sleep 1
					chmod 666 "${WAYPIPE_CTRL_SOCKET}" 2>&1 || true
					cat /tmp/waypipe-client.log 2>/dev/null || true
				'`).Privileged().Environ("WAYLAND_DISPLAY", egenv.String("", "WAYLAND_HOST_SOCKET")),
				shell.Env().New(`sh -c '
					nohup waypipe -s "${WAYPIPE_CTRL_SOCKET}" --display "${WAYLAND_DISPLAY}" server -- sleep infinity > /tmp/waypipe-server.log 2>&1 &
					sleep 1
					cat /tmp/waypipe-server.log 2>/dev/null || true
				'`),
			),
		),
	)
}
