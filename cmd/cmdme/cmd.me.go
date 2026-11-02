// Package cmdme manages the profile of the authenticated user.
package cmdme

type Cmd struct {
	Ls      Ls      `cmd:"" name:"ls" help:"show the current profile and every profile associated with your ssh key"`
	Edit    Edit    `cmd:"" name:"edit" help:"edit your profile"`
	Enable  Enable  `cmd:"" name:"enable" help:"enable a profile associated with your ssh key"`
	Disable Disable `cmd:"" name:"disable" help:"disable a profile associated with your ssh key, the current profile cannot be disabled"`
}
