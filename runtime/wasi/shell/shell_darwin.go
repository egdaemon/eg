package shell

import "os/user"

func defaultgroup(u *user.User) string {
	return "staff"
}
