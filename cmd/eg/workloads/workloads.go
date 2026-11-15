// Package workloads provides the commands that inspect the workloads of repositories.
package workloads

type Cmd struct {
	Detect detectCmd `cmd:"" name:"detect" help:"detect the workloads of repositories and the commands that run them, as the editor extension needs them"`
}
