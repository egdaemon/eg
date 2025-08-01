package shell

import (
	"os/user"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
)

func defaultgroup(u *user.User) string {
	return envx.String(u.Username, eg.EnvComputeDefaultGroup)
}
