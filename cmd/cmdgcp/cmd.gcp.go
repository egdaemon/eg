// Package cmdgcp manages the gcp workload identity federation connections that let
// runners present egdaemon's identity to a customer's gcp project.
package cmdgcp

type Cmd struct {
	WIF WIF `cmd:"" name:"wif" help:"manage workload identity federation connections"`
}

type WIF struct {
	Create Create `cmd:"" name:"create" help:"create a connection"`
	List   List   `cmd:"" name:"ls" help:"list connections"`
	Edit   Edit   `cmd:"" name:"edit" help:"edit a connection"`
	Remove Remove `cmd:"" name:"rm" help:"remove a connection"`
}
