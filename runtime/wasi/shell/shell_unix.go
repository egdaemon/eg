//go:build !darwin

package shell

import "os/user"

func defaultgroup(u *user.User) string {
	return u.Username
}
