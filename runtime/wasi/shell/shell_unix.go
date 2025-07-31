//go:build !darwin

package shell

import (
	"log"
	"os/user"

	"github.com/davecgh/go-spew/spew"
)

func defaultgroup(u *user.User) string {
	log.Println("defaultgroup unix", spew.Sdump(u))
	return u.Username
}
