package cmdaws

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/awswif"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sshx"
)

// Selector selects the connection, defaults to the account id of the current profile.
type Selector struct {
	cmdopts.VCS `embed:""`
	Label       string `arg:"" optional:"" name:"label" help:"label of the connection, defaults to the account id (--account-id)"`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

type Create struct {
	Selector
	RoleARN     string `name:"role-arn" help:"arn of the iam role workloads assume: arn:aws:iam::<account>:role/<name>" required:""`
	Region      string `name:"region" help:"optional default region of the workloads"`
	Audience    string `name:"audience" help:"audience of the identity tokens, must match the client id of the iam identity provider" default:"sts.amazonaws.com"`
	Description string `name:"description" help:"optional description"`
}

func (t Create) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	// the management routes require the user management permission.
	aid, err := gctx.Identity().Account(tlsc.DefaultClient(), signer, t.VcsURI)
	if err != nil {
		return err
	}

	c := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

	// an empty label defaults to the account id on the server.
	conn, err := awswif.Create(gctx.Context, c, &awswif.AWSWIFConnection{
		Label:       t.Label,
		RoleArn:     t.RoleARN,
		Region:      t.Region,
		Audience:    t.Audience,
		Description: t.Description,
	})
	if err != nil {
		return err
	}

	fmt.Printf("created %s\n", conn.GetLabel())
	return nil
}

type List struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Query       string `name:"query" short:"q" help:"only list connections whose label, audience, or description contain the text"`
}

func (t List) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	// the management routes require the user management permission.
	aid, err := gctx.Identity().Account(tlsc.DefaultClient(), signer, t.VcsURI)
	if err != nil {
		return err
	}

	c := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "LABEL\tROLE\tREGION\tAUDIENCE\tDESCRIPTION")

	// follow next until the last page, which resets the offset to zero.
	for offset := uint64(0); ; {
		rsp, err := awswif.Search(gctx.Context, c, t.Query, offset)
		if err != nil {
			return err
		}

		for _, i := range rsp.GetItems() {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", i.GetLabel(), i.GetRoleArn(), i.GetRegion(), i.GetAudience(), i.GetDescription())
		}

		if offset = rsp.GetNext().GetOffset(); offset == 0 {
			break
		}
	}

	return w.Flush()
}

type Edit struct {
	Selector
	RoleARN     string `name:"role-arn" help:"new iam role arn"`
	Region      string `name:"region" help:"new default region" xor:"region"`
	ClearRegion bool   `name:"clear-region" help:"stop setting a default region" xor:"region"`
	Audience    string `name:"audience" help:"new audience of the identity tokens"`
	Description string `name:"description" help:"new description"`
}

func (t Edit) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	// the management routes require the user management permission.
	aid, err := gctx.Identity().Account(tlsc.DefaultClient(), signer, t.VcsURI)
	if err != nil {
		return err
	}

	c := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

	var req awswif.AWSWIFUpdateRequest
	if t.RoleARN != "" {
		req.RoleArn = &t.RoleARN
	}
	if t.Region != "" {
		req.Region = &t.Region
	}
	if t.ClearRegion {
		empty := ""
		req.Region = &empty
	}
	if t.Audience != "" {
		req.Audience = &t.Audience
	}
	if t.Description != "" {
		req.Description = &t.Description
	}

	if req.RoleArn == nil && req.Region == nil && req.Audience == nil && req.Description == nil {
		return errorsx.New("nothing to change, provide at least one of --role-arn, --region, --clear-region, --audience, --description")
	}

	l := langx.FirstNonZero(t.Label, gctx.AccountID)
	if l == "" {
		return errorsx.New("a label or --account-id is required")
	}

	if _, err = awswif.Update(gctx.Context, c, l, &req); err != nil {
		return err
	}

	fmt.Printf("updated %s\n", l)
	return nil
}

type Remove struct {
	Selector
}

func (t Remove) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	// the management routes require the user management permission.
	aid, err := gctx.Identity().Account(tlsc.DefaultClient(), signer, t.VcsURI)
	if err != nil {
		return err
	}

	c := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

	l := langx.FirstNonZero(t.Label, gctx.AccountID)
	if l == "" {
		return errorsx.New("a label or --account-id is required")
	}

	if err = awswif.Remove(gctx.Context, c, l); err != nil {
		return err
	}

	fmt.Printf("removed %s\n", l)
	return nil
}

// OIDC prints what the aws account needs to trust the workloads of the account.
type OIDC struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t OIDC) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	// the route requires the user management permission.
	aid, err := gctx.Identity().Account(tlsc.DefaultClient(), signer, t.VcsURI)
	if err != nil {
		return err
	}

	c := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

	rsp, err := awswif.Issuer(gctx.Context, c)
	if err != nil {
		return err
	}

	fmt.Printf("issuer: %s\nsubject: %s\n", rsp.GetIssuerUrl(), rsp.GetSubject())
	return nil
}
