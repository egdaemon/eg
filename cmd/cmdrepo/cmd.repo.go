// Package cmdrepo commands for the repository in the current workspace and the identity used with it.
package cmdrepo

type Cmd struct {
	Whoami  Whoami  `cmd:"" name:"whoami" help:"display the current credentials, and the account and profile used for this repository"`
	Default Default `cmd:"" name:"default" help:"manage the defaults for this repository"`
}

type Default struct {
	Account DefaultAccount `cmd:"" name:"account" help:"set the account used with this repository, detects it when an id is not provided"`
}
