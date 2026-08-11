package runners

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
)

// configures a local machine run to connect to the host's wayland compositor.
//
// The upstream socket is bind-mounted into eg.DefaultRuntimeDirectory, the
// same runtime-dir mount already used for things like the ssh-agent socket --
// bindfs can't proxy live sockets (only regular files), so ownership can't be
// remapped the way AgentOptionGcloudCredentials remaps the gcloud config dir.
// The mounted socket itself is still root-owned from the unprivileged
// container user's perspective; the caller is expected to run a tool like
// waypipe that speaks the Wayland protocol itself (needed for fd-passing -- a
// plain byte-relay proxy like socat won't work for real clients) to give the
// unprivileged user its own usable socket.
func AgentOptionWayland(ctx context.Context, envb *envx.Builder) AgentOption {
	display := envx.String("wayland-0", "WAYLAND_DISPLAY")

	// WAYLAND_DISPLAY is conventionally a bare name joined against
	// XDG_RUNTIME_DIR, but wayland/libwayland clients also accept it being
	// set to an absolute path directly -- honor both.
	sock := display
	if !filepath.IsAbs(display) {
		xdgruntime := envx.String("", "XDG_RUNTIME_DIR")
		if xdgruntime == "" {
			log.Println("XDG_RUNTIME_DIR is not set, unable to locate wayland socket")
			return AgentOptionNoop
		}
		sock = filepath.Join(xdgruntime, display)
	}

	if _, err := os.Stat(sock); err != nil {
		log.Println("wayland socket is not available", err)
		return AgentOptionNoop
	}

	// WAYLAND_DISPLAY is the well-known, unprivileged-writable socket that
	// in-container apps connect to (e.g. via a waypipe server); the real
	// host socket bind-mounted below is root-owned from the unprivileged
	// container user's perspective, so it's exposed separately as
	// WAYLAND_HOST_SOCKET for whatever privileged process (e.g. a waypipe
	// client) bridges the two. WAYPIPE_CTRL_SOCKET is the rendezvous socket
	// between that privileged client and the unprivileged server; it must
	// live somewhere egd can create it, hence /tmp rather than
	// eg.DefaultRuntimeDirectory. GDK_BACKEND pins GTK apps to wayland so
	// they don't silently fall back to a nonexistent X11 display.
	envb.Var("WAYLAND_DISPLAY", "/tmp/wayland.sock")
	envb.Var("WAYLAND_HOST_SOCKET", eg.DefaultRuntimeDirectory("wayland.host.sock"))
	envb.Var("WAYPIPE_CTRL_SOCKET", "/tmp/waypipe.sock")
	envb.Var("GDK_BACKEND", "wayland")

	return AgentOptionVolumes(
		AgentMountReadWrite(
			sock,
			eg.DefaultRuntimeDirectory("wayland.host.sock"),
		),
	)
}
