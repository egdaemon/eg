//go:build !darwin

package podmanx

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/userx"
)

func DefaultSocket() string {
	u := userx.CurrentUserOrDefault(userx.Root())
	upath := filepath.Join("/var", "run", "user", u.Uid, "podman", "podman.sock")
	rpath := filepath.Join("/run", "podman", "podman.sock")
	log.Println("DERP DERP", upath, rpath)
	socketpath := stringsx.DefaultIfBlank(fsx.LocateFirst(upath, rpath), upath)
	return fmt.Sprintf("unix://%s", socketpath)
}
