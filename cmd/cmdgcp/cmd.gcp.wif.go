package cmdgcp

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"text/tabwriter"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
)

// Selector selects the connection, defaults to the account id of the current profile.
type Selector struct {
	Label      string `arg:"" optional:"" name:"label" help:"label of the connection, defaults to the account id of the current profile"`
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

type Create struct {
	Selector
	Audience    string `name:"audience" help:"workload identity pool provider: //iam.googleapis.com/projects/<number>/locations/global/workloadIdentityPools/<pool>/providers/<provider>" required:""`
	Impersonate string `name:"impersonate" help:"optional service account email to impersonate after the token exchange"`
	Description string `name:"description" help:"optional description"`
}

func (t Create) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	c, err := newClient(gctx.Context, gctx, tlsc, t.SSHKeyPath)
	if err != nil {
		return err
	}

	var rsp GCPWIFCreateResponse
	if err = c.do(gctx.Context, "POST", "", nil, &GCPWIFCreateRequest{Connection: &GCPWIFConnection{
		Label:                     c.label(t.Label),
		Audience:                  t.Audience,
		ImpersonateServiceAccount: t.Impersonate,
		Description:               t.Description,
	}}, &rsp); err != nil {
		return errorsx.Wrap(err, "unable to create connection")
	}

	fmt.Printf("created %s\n", rsp.GetConnection().GetLabel())
	return nil
}

type List struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Query      string `name:"query" short:"q" help:"only list connections whose label, audience, or description contain the text"`
}

func (t List) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	c, err := newClient(gctx.Context, gctx, tlsc, t.SSHKeyPath)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "LABEL\tAUDIENCE\tIMPERSONATE\tDESCRIPTION")

	// follow next until the last page, which resets the offset to zero.
	for offset := uint64(0); ; {
		var rsp GCPWIFSearchResponse
		if err = c.do(gctx.Context, "GET", "", url.Values{"offset": {strconv.FormatUint(offset, 10)}, "query": {t.Query}}, nil, &rsp); err != nil {
			return errorsx.Wrap(err, "unable to list connections")
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
	c, err := newClient(gctx.Context, gctx, tlsc, t.SSHKeyPath)
	if err != nil {
		return err
	}

	var req GCPWIFUpdateRequest
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

	label := c.label(t.Label)
	if err = c.do(gctx.Context, "PATCH", label, nil, &req, &GCPWIFUpdateResponse{}); err != nil {
		return errorsx.Wrap(err, "unable to edit connection")
	}

	fmt.Printf("updated %s\n", label)
	return nil
}

type Remove struct {
	Selector
}

func (t Remove) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) error {
	c, err := newClient(gctx.Context, gctx, tlsc, t.SSHKeyPath)
	if err != nil {
		return err
	}

	label := c.label(t.Label)
	if err = c.do(gctx.Context, "DELETE", label, nil, nil, nil); err != nil {
		return errorsx.Wrap(err, "unable to remove connection")
	}

	fmt.Printf("removed %s\n", label)
	return nil
}
