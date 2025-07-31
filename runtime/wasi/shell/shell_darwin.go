package shell

import (
	"log"
	"os/user"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/internal/stringsx"
)

func defaultgroup(u *user.User) string {
	log.Println("defaultgroup darwin", spew.Sdump(u))
	if stringsx.Present(u.Username) {
		return "staff"
	}

	return ""
}
