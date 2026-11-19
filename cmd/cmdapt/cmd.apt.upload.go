package cmdapt

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/egdaemon/eg/apt"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
)

type Upload struct {
	target  `embed:""`
	Debs    []string `arg:"" name:"deb" help:"debian packages to upload" type:"existingfile"`
	Publish bool     `name:"publish" help:"publish the repository once every package uploaded successfully"`
	Retries int      `name:"retries" help:"number of times a concurrent publish is retried" default:"3"`
}

func (t Upload) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) error {
	c, err := t.ci(gctx, tlsc, ident)
	if err != nil {
		return err
	}

	if err = upload(gctx, c, os.Stdout, t.Debs...); err != nil {
		return err
	}

	if !t.Publish {
		return nil
	}

	return publish(gctx, c, os.Stdout, t.Retries, apt.Backoff(time.Second))
}

// upload every package reporting the outcome of each, the failures are returned together once
// every package was attempted.
func upload(gctx *cmdopts.Global, c apt.Client, dst io.Writer, debs ...string) (err error) {
	for _, deb := range debs {
		out, cause := c.Upload(gctx.Context, deb)
		switch {
		case cause == nil && out.Staged:
			fmt.Fprintf(dst, "staged %s %s %s\n", deb, out.Filename, out.SHA256)
		case cause == nil:
			fmt.Fprintf(dst, "unchanged %s %s %s\n", deb, out.Filename, out.SHA256)
		case apt.Status(cause, http.StatusConflict):
			fmt.Fprintf(dst, "conflict %s\n", deb)
			err = errors.Join(err, fmt.Errorf("%s: a different package is already published at the same pool path: %w", deb, cause))
		case apt.Status(cause, http.StatusBadRequest):
			fmt.Fprintf(dst, "invalid %s\n", deb)
			err = errors.Join(err, fmt.Errorf("%s: not a valid debian package: %w", deb, cause))
		default:
			fmt.Fprintf(dst, "failed %s\n", deb)
			err = errors.Join(err, explain(cause, "upload "+deb))
		}
	}

	return err
}

type Publish struct {
	target  `embed:""`
	Retries int `name:"retries" help:"number of times a concurrent publish is retried" default:"3"`
}

func (t Publish) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) error {
	c, err := t.ci(gctx, tlsc, ident)
	if err != nil {
		return err
	}

	return publish(gctx, c, os.Stdout, t.Retries, apt.Backoff(time.Second))
}

func publish(gctx *cmdopts.Global, c apt.Client, dst io.Writer, retries int, backoff func(int) time.Duration) error {
	vid, err := c.PublishRetry(gctx.Context, retries, backoff)
	if apt.Status(err, http.StatusConflict) {
		return fmt.Errorf("unable to publish, another publish is in progress after %d retries: %w", retries, err)
	} else if err != nil {
		return explain(err, "publish")
	}

	_, err = fmt.Fprintln(dst, vid)
	return err
}
