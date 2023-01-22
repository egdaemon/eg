package userx

import (
	"log"
	"os/user"
)

// CurrentUserOrDefault returns the current user or the default configured user.
// (usually root)
func CurrentUserOrDefault(d user.User) (result *user.User) {
	var (
		err error
	)

	if result, err = user.Current(); err != nil {
		log.Println("failed to retrieve current user, using default", err)
		tmp := d
		return &tmp
	}

	return result
}
