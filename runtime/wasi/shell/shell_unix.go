package shell

import (
	"log"
	"os/user"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
)

func defaultgroup(u *user.User) string {
	log.Println("defaultgroup unix", spew.Sdump(u), envx.String(u.Username, eg.EnvComputeDefaultGroup))
	return envx.String(u.Username, eg.EnvComputeDefaultGroup)
}
