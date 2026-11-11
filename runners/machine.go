package runners

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/workspaces"
	"github.com/opencontainers/go-digest"
)

// Machine runs a workload's modules in a machine booted from an egiso, with the workspace's
// directories mounted where a container mounts them as volumes.
type Machine struct {
	ws      workspaces.Context
	archive *egiso.Archive
	upper   *os.Root
	vm      *egvm.VM
}

// NewMachine boots a machine from the egiso uri names, as egiso.OpenURI opens it, under a writable
// upper kept in the workspace, and returns once its systemd finished booting.
func NewMachine(ctx context.Context, c *http.Client, ws workspaces.Context, uri string, console io.Writer, options ...egvm.Option) (_ *Machine, err error) {
	var (
		t        = &Machine{ws: ws}
		upperdir string
		egbin    string
		users    []egiso.Account
		groups   []egiso.Account
	)
	defer func() {
		if err != nil {
			errorsx.Log(t.Close())
		}
	}()

	if t.archive, err = egiso.OpenURI(ctx, c, uri); err != nil {
		return nil, err
	}

	if upperdir, err = os.MkdirTemp(ws.Root, "vm.*"); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine's upper")
	}

	if t.upper, err = os.OpenRoot(upperdir); err != nil {
		return nil, errorsx.Wrap(err, "unable to open the machine's upper")
	}

	if users, groups, err = t.archive.Accounts(); err != nil {
		return nil, err
	}

	// containers mount the eg running the command at egbin; machines share directories, not files.
	if egbin, err = exec.LookPath(os.Args[0]); err != nil {
		return nil, errorsx.Wrap(err, "unable to locate eg")
	}
	if err = iox.Copy(egbin, filepath.Join(ws.RuntimeDir, eg.BinaryBin)); err != nil {
		return nil, errorsx.Wrap(err, "unable to place egbin")
	}

	// the machine has the image's own accounts, so the image's files keep their owners.
	root := egiso.NewSysFS(egiso.NewUpper(egiso.Tree{}.Mount(".", t.archive, egiso.Identity(users), egiso.Identity(groups)), t.upper), egiso.Default, egiso.Default)

	if t.vm, err = egvm.Boot(egvm.New(root, append([]egvm.Option{
		egvm.OptionAccounts(users, groups),
		egvm.OptionMountReadWrite(ws.CacheDir, eg.DefaultMountRoot(eg.CacheDirectory)),
		egvm.OptionMountReadWrite(ws.RuntimeDir, eg.DefaultMountRoot(eg.RuntimeDirectory)),
		egvm.OptionMountReadWrite(ws.WorkingDir, eg.DefaultMountRoot(eg.WorkingDirectory)),
		egvm.OptionMountReadWrite(ws.WorkspaceDir, eg.DefaultMountRoot(eg.WorkspaceDirectory)),
	}, options...)...), console); err != nil {
		return nil, err
	}

	if err = t.vm.Exec(ctx, &execproxy.ExecRequest{Cmd: "systemctl", Arguments: []string{"is-system-running", "--wait"}, Dir: "/"}); err != nil {
		return nil, errorsx.Wrap(err, "the machine did not finish booting")
	}

	return t, nil
}

// BuildRootEgiso builds the root container as BuildRootContainerPath does and returns the uri of an
// egiso of its filesystem, which it keeps in cache under the image's id so it builds one per image.
func BuildRootEgiso(ctx context.Context, cache, dir, path string, options ...string) (_ string, err error) {
	var (
		id     string
		cid    string
		tar    *os.File
		st     os.FileInfo
		out    *os.File
		toc    digest.Digest
		cached []byte
	)

	if err = BuildRootContainerPath(ctx, dir, path, options...); err != nil {
		return "", err
	}

	if id, err = execx.String(ctx, "podman", "image", "inspect", "--format", "{{.Id}}", "eg"); err != nil {
		return "", errorsx.Wrap(err, "unable to identify the root container")
	}
	location := filepath.Join(cache, strings.TrimSpace(id)+".egiso")

	// the table of contents is written once its egiso is complete.
	if cached, err = os.ReadFile(location + ".toc"); err == nil {
		return location + "#" + string(cached), nil
	}

	if err = os.MkdirAll(cache, 0700); err != nil {
		return "", errorsx.Wrap(err, "unable to create the egiso cache")
	}

	if tar, err = os.CreateTemp(cache, "root.*.tar"); err != nil {
		return "", errorsx.Wrap(err, "unable to create the root container's archive")
	}
	defer func() { err = errorsx.Compact(err, tar.Close(), os.Remove(tar.Name())) }()

	if cid, err = execx.String(ctx, "podman", "create", "eg"); err != nil {
		return "", errorsx.Wrap(err, "unable to create the root container")
	}
	cid = strings.TrimSpace(cid)
	defer func() { err = errorsx.Compact(err, exec.Command("podman", "rm", cid).Run()) }()

	if err = exec.CommandContext(ctx, "podman", "export", "-o", tar.Name(), cid).Run(); err != nil {
		return "", errorsx.Wrap(err, "unable to export the root container")
	}

	if st, err = tar.Stat(); err != nil {
		return "", err
	}

	if out, err = os.Create(location); err != nil {
		return "", errorsx.Wrap(err, "unable to create the egiso")
	}
	defer out.Close()

	if toc, err = egiso.Build(io.NewSectionReader(tar, 0, st.Size()), out); err != nil {
		return "", errorsx.Wrap(err, "unable to build the egiso")
	}

	if err = os.WriteFile(location+".toc", []byte(toc), 0600); err != nil {
		return "", errorsx.Wrap(err, "unable to record the egiso's table of contents")
	}

	return location + "#" + toc.String(), nil
}

// Module runs the module at path in the machine, as c8sproxy.PodmanModule runs it in a container,
// with the workspace's environment file and environ added to it.
func (t *Machine) Module(ctx context.Context, path string, environ []string, options ...egvm.ExecOption) (err error) {
	var (
		env       []string
		verbosity string
	)

	if x := envx.Int(0, eg.EnvComputeLoggingVerbosity); x > 0 {
		verbosity = fmt.Sprintf("-%s", strings.Repeat("v", x))
	}

	if err = iox.Copy(path, filepath.Join(t.ws.RuntimeDir, eg.ModuleBin)); err != nil {
		return errorsx.Wrap(err, "unable to place the module")
	}

	if env, err = envx.Build().FromPath(filepath.Join(t.ws.RuntimeDir, eg.EnvironFile)).FromEnviron(environ...).Environ(); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

	// TODO REVISIT using t.ws.RuntimeDir as moduledir, as containers do.
	return t.vm.Exec(ctx, &execproxy.ExecRequest{
		Cmd:         envx.String("eg", eg.EnvComputeBin),
		Arguments:   slicesx.Filter(stringsx.Present, verbosity, "module", "--directory", eg.DefaultWorkingDirectory(), "--moduledir", t.ws.RuntimeDir, eg.ModuleMount()),
		Dir:         "/",
		Environment: env,
	}, options...)
}

// Close shuts the machine down and releases its egiso.
func (t *Machine) Close() error {
	var errs []error
	if t.vm != nil {
		errs = append(errs, t.vm.Close())
	}
	if t.upper != nil {
		errs = append(errs, t.upper.Close())
	}
	if t.archive != nil {
		errs = append(errs, t.archive.Close())
	}
	return errorsx.Compact(errs...)
}
