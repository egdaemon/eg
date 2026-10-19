package cmdgcp

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/gcpwif"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sshx"
)

// Selector selects the connection, defaults to the account id of the current profile.
type Selector struct {
	Label      string `arg:"" optional:"" name:"label" help:"label of the connection, defaults to the account id (--account-id)"`
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

type Create struct {
	Selector
	Audience    string `name:"audience" help:"workload identity pool provider: //iam.googleapis.com/projects/<number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>" required:""`
	Impersonate string `name:"impersonate" help:"optional service account email to impersonate after the token exchange"`
	Description string `name:"description" help:"optional description"`
}

func (t Create) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	// the management routes require the user management permission.
	c := authn.New(gctx.Context, tlsc.DefaultClient(), signer, gctx.AccountID, compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

	// an empty label defaults to the account id on the server.
	conn, err := gcpwif.Create(gctx.Context, c, &gcpwif.GCPWIFConnection{
		Label:                     t.Label,
		Audience:                  t.Audience,
		ImpersonateServiceAccount: t.Impersonate,
		Description:               t.Description,
	})
	if err != nil {
		return err
	}

	fmt.Printf("created %s\n", conn.GetLabel())
	return nil
}

type List struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Query      string `name:"query" short:"q" help:"only list connections whose label, audience, or description contain the text"`
}

func (t List) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	// the management routes require the user management permission.
	c := authn.New(gctx.Context, tlsc.DefaultClient(), signer, gctx.AccountID, compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "LABEL\tAUDIENCE\tIMPERSONATE\tDESCRIPTION")

	// follow next until the last page, which resets the offset to zero.
	for offset := uint64(0); ; {
		rsp, err := gcpwif.Search(gctx.Context, c, t.Query, offset)
		if err != nil {
			return err
		}

		for _, i := range rsp.GetItems() {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", i.GetLabel(), i.GetAudience(), i.GetImpersonateServiceAccount(), i.GetDescription())
		}

		if offset = rsp.GetNext().GetOffset(); offset == 0 {
			break
		}
	}

	return w.Flush()
}

type Edit struct {
	Selector
	Audience         string `name:"audience" help:"new workload identity pool provider"`
	Impersonate      string `name:"impersonate" help:"new service account email to impersonate" xor:"impersonate"`
	ClearImpersonate bool   `name:"clear-impersonate" help:"stop impersonating a service account" xor:"impersonate"`
	Description      string `name:"description" help:"new description"`
}

func (t Edit) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	// the management routes require the user management permission.
	c := authn.New(gctx.Context, tlsc.DefaultClient(), signer, gctx.AccountID, compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

	var req gcpwif.GCPWIFUpdateRequest
	if t.Audience != "" {
		req.Audience = &t.Audience
	}
	if t.Impersonate != "" {
		req.ImpersonateServiceAccount = &t.Impersonate
	}
	if t.ClearImpersonate {
		empty := ""
		req.ImpersonateServiceAccount = &empty
	}
	if t.Description != "" {
		req.Description = &t.Description
	}

	if req.Audience == nil && req.ImpersonateServiceAccount == nil && req.Description == nil {
		return errorsx.New("nothing to change, provide at least one of --audience, --impersonate, --clear-impersonate, --description")
	}

	l := langx.FirstNonZero(t.Label, gctx.AccountID)
	if l == "" {
		return errorsx.New("a label or --account-id is required")
	}

	if _, err = gcpwif.Update(gctx.Context, c, l, &req); err != nil {
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
	c := authn.New(gctx.Context, tlsc.DefaultClient(), signer, gctx.AccountID, compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

	l := langx.FirstNonZero(t.Label, gctx.AccountID)
	if l == "" {
		return errorsx.New("a label or --account-id is required")
	}

	if err = gcpwif.Remove(gctx.Context, c, l); err != nil {
		return err
	}

	fmt.Printf("removed %s\n", l)
	return nil
}
