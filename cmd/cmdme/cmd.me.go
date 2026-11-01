// Package cmdme manages the profile of the authenticated user.
package cmdme

type Cmd struct {
	Ls   Ls   `cmd:"" name:"ls" help:"show the current profile and every profile associated with your ssh key"`
	Edit Edit `cmd:"" name:"edit" help:"edit your profile"`
}
