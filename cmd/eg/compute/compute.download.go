package compute

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
)

type download struct {
	Logs      downloadlogs      `cmd:"" help:"download the logs of a workload by id"`
	Analytics downloadanalytics `cmd:"" help:"download the analytics database of a workload by id directly from its runner"`
}

type downloadopts struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Output      string `name:"output" short:"o" help:"file to store the download to, '-' writes to stdout. defaults to ./{id}.{logs|analytics.db}"`
	ID          string `arg:"" name:"id" help:"id of the enqueued workload, printed when it was uploaded"`
}

func (t downloadopts) client(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (_ *http.Client, err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return nil, err
	}

	aid, err := gctx.Identity().Account(tlsc.DefaultClient(), signer, t.VcsURI)
	if err != nil {
		return nil, err
	}

	return authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource), nil
}

type downloadlogs struct {
	downloadopts `embed:""`
}

func (t downloadlogs) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	chttp, err := t.client(gctx, tlsc)
	if err != nil {
		return err
	}

	return downloadto(t.Output, t.ID+".log", func(dst io.Writer) error {
		return computeapi.DownloadLogs(gctx.Context, chttp, t.ID, dst)
	})
}

type downloadanalytics struct {
	downloadopts `embed:""`
}

func (t downloadanalytics) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	chttp, err := t.client(gctx, tlsc)
	if err != nil {
		return err
	}

	return downloadto(t.Output, t.ID+".analytics.db", func(dst io.Writer) error {
		return computeapi.DownloadAnalytics(gctx.Context, chttp, t.ID, dst)
	})
}

// downloadto runs fetch writing into the output, '-' being stdout and blank the default name. files are written
// beside their destination and renamed into place once complete so failures never leave partial downloads.
func downloadto(output string, defaultname string, fetch func(dst io.Writer) error) (err error) {
	if output == "-" {
		return fetch(os.Stdout)
	}

	if stringsx.Blank(output) {
		output = defaultname
	}

	tmp, err := os.CreateTemp(filepath.Dir(output), ".download-*")
	if err != nil {
		return errorsx.Wrap(err, "unable to create download")
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if err = fetch(tmp); err != nil {
		errorsx.Log(tmp.Close())
		return err
	}

	if err = tmp.Close(); err != nil {
		return errorsx.Wrap(err, "unable to write download")
	}

	if err = os.Chmod(tmp.Name(), 0600); err != nil {
		return errorsx.Wrap(err, "unable to write download")
	}

	if err = os.Rename(tmp.Name(), output); err != nil {
		return errorsx.Wrap(err, "unable to store download")
	}

	log.Println("downloaded", output)
	return nil
}
