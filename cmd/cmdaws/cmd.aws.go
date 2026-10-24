// Package cmdaws manages the aws web identity federation connections that let
// runners present egdaemon's identity to a customer's aws account.
package cmdaws

type Cmd struct {
	WIF  WIF  `cmd:"" name:"wif" help:"manage web identity federation connections"`
	OIDC OIDC `cmd:"" name:"oidc" help:"show the issuer and subject your aws account trusts"`
}

type WIF struct {
	Create Create `cmd:"" name:"create" help:"create a connection"`
	List   List   `cmd:"" name:"ls" help:"list connections"`
	Edit   Edit   `cmd:"" name:"edit" help:"edit a connection"`
	Remove Remove `cmd:"" name:"rm" help:"remove a connection"`
}
