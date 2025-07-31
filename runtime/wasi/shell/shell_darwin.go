package shell

import (
	"os/user"

	"github.com/egdaemon/eg/internal/stringsx"
)

func defaultgroup(u *user.User) string {
	if stringsx.Present(u.Username) {
		return "staff"
	}

	return ""
}
