package runners

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/moby/sys/user"
	"github.com/opencontainers/go-digest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Machine runs a workload's modules in a machine booted from an egiso, with the workspace's
// directories mounted where a container mounts them as volumes. Its modules run modules of their own
// in machines it boots beside it, through eg.MachineSocket.
type Machine struct {
	ws        workspaces.Context
	archive   *egiso.Archive
	upper     *os.Root
	vm        *egvm.VM
	environ   []string
	moduleenv []string
	siblings  *grpc.Server
	dir       string
}

// MachineOption changes the machine NewMachine boots, as an AgentOption changes a container.
type MachineOption func(*machineconfig)

type machineconfig struct {
	vm       []egvm.Option
	overlays []attachment
	sockets  []attachment
	environ  []string
}

// attachment is a host directory or socket, host, the machine sees at path.
type attachment struct {
	host string
	path string
}

// MachineOptionVM boots the machine with the options.
func MachineOptionVM(options ...egvm.Option) MachineOption {
	return func(c *machineconfig) {
		c.vm = append(c.vm, options...)
	}
}

// MachineOptionOverlay mounts the host directory dir at path in the machine, with the changes the
// machine makes within it discarded, as AgentMountOverlay does.
func MachineOptionOverlay(dir string, path string) MachineOption {
	return func(c *machineconfig) {
		c.overlays = append(c.overlays, attachment{host: dir, path: path})
	}
}

// MachineOptionSocket makes the host's unix socket host reachable in the machine at path, which must be
// within an overlay: the root can not hold a socket.
func MachineOptionSocket(host string, path string) MachineOption {
	return func(c *machineconfig) {
		c.sockets = append(c.sockets, attachment{host: host, path: path})
	}
}

// MachineOptionEnviron adds environ to the environment the machine runs its module with.
func MachineOptionEnviron(environ ...string) MachineOption {
	return func(c *machineconfig) {
		c.environ = append(c.environ, environ...)
	}
}

// MachineOptionNoop changes nothing.
func MachineOptionNoop(*machineconfig) {}

// NewMachine boots a machine from the egiso uri names, as egiso.OpenURI opens it, under a writable
// upper kept in the workspace, and returns once its systemd finished booting. The machine runs the
// eg the workspace's runtime directory holds as egbin.
func NewMachine(ctx context.Context, c *http.Client, ws workspaces.Context, uri string, console io.Writer, options ...MachineOption) (_ *Machine, err error) {
	var (
		t        = &Machine{ws: ws}
		config   = langx.Clone(machineconfig{}, options...)
		upperdir string
		copied   string
		users    []egiso.Account
		groups   []egiso.Account
		siblings net.Listener
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

	if t.environ, err = environ(t.archive); err != nil {
		return nil, err
	}

	// the machine's tmpfiles empty /eg.mnt of all but its mounts, so an overlay is a copy in the
	// workspace mounted in its place.
	for _, o := range config.overlays {
		if copied, err = os.MkdirTemp(ws.Root, "overlay.*"); err != nil {
			return nil, errorsx.Wrapf(err, "unable to create the overlay of %s", o.host)
		}
		if err = fsx.CloneTree(ctx, copied, filepath.Base(o.host), os.DirFS(filepath.Dir(o.host))); err != nil {
			return nil, errorsx.Wrapf(err, "unable to copy %s", o.host)
		}
		config.vm = append(config.vm, egvm.OptionMountReadWrite(copied, o.path))
	}

	if t.dir, err = os.MkdirTemp("", "egmachine.*"); err != nil {
		return nil, errorsx.Wrap(err, "unable to create the machine's directory")
	}
	if siblings, err = net.Listen("unix", filepath.Join(t.dir, "machine.socket")); err != nil {
		return nil, errorsx.Wrap(err, "unable to listen for the machine's modules")
	}
	t.siblings = grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	c8s.RegisterProxyServer(t.siblings, machines{c: c, ws: ws, uri: uri, console: console, options: options})
	go func() {
		errorsx.Log(errorsx.Wrap(t.siblings.Serve(siblings), "unable to serve the machine's modules"))
	}()
	config.sockets = append(config.sockets, attachment{host: siblings.Addr().String(), path: eg.MachineSocket})
	t.moduleenv = config.environ

	// the machine has the image's own accounts, so the image's files keep their owners. The host user
	// owns what it writes in the workspace's directories, except the runtime directory, whose sockets
	// the machine binds.
	root := egiso.NewSysFS(egiso.NewUpper(egiso.Tree{}.Mount(".", t.archive, egiso.Identity(users), egiso.Identity(groups)), t.upper), egiso.Default, egiso.Default)

	if t.vm, err = egvm.Boot(egvm.New(root, append([]egvm.Option{
		egvm.OptionAccounts(users, groups),
		egvm.OptionMountOwned(ws.CacheDir, eg.DefaultMountRoot(eg.CacheDirectory)),
		egvm.OptionMountReadWrite(ws.RuntimeDir, eg.DefaultMountRoot(eg.RuntimeDirectory)),
		egvm.OptionMountOwned(ws.WorkingDir, eg.DefaultMountRoot(eg.WorkingDirectory)),
		egvm.OptionMountOwned(ws.WorkspaceDir, eg.DefaultMountRoot(eg.WorkspaceDirectory)),
	}, config.vm...)...), console); err != nil {
		return nil, err
	}

	if err = t.vm.Exec(ctx, &execproxy.ExecRequest{Cmd: "systemctl", Arguments: []string{"is-system-running", "--wait"}, Dir: "/"}); err != nil {
		return nil, errorsx.Wrap(err, "the machine did not finish booting")
	}

	for _, s := range config.sockets {
		if err = t.vm.Forward(ctx, s.host, s.path); err != nil {
			return nil, errorsx.Wrapf(err, "unable to forward %s", s.host)
		}
	}

	return t, nil
}

// BuildRootEgiso builds the root container as BuildRootContainerPath does and returns the uri of an
// egiso of its filesystem, with the image's environment as its /etc/environment. It keeps the egiso
// of the latest image in cache, in a directory named for the image's id, so it builds one per image.
func BuildRootEgiso(ctx context.Context, cache, dir, path string, options ...string) (_ string, err error) {
	var (
		id      string
		cid     string
		environ string
		tar     *os.File
		st      os.FileInfo
		out     *os.File
		toc     digest.Digest
		cached  []byte
	)

	if err = BuildRootContainerPath(ctx, dir, path, options...); err != nil {
		return "", err
	}

	if id, err = execx.String(ctx, "podman", "image", "inspect", "--format", "{{.Id}}", "eg"); err != nil {
		return "", errorsx.Wrap(err, "unable to identify the root container")
	}
	imagedir := filepath.Join(cache, strings.TrimSpace(id))
	location := filepath.Join(imagedir, "root.egiso")

	// the table of contents is written once its egiso is complete.
	if cached, err = os.ReadFile(filepath.Join(imagedir, "toc")); err == nil {
		return location + "#" + string(cached), nil
	}

	if err = os.MkdirAll(imagedir, 0700); err != nil {
		return "", errorsx.Wrap(err, "unable to create the egiso cache")
	}

	if tar, err = os.CreateTemp(imagedir, "root.*.tar"); err != nil {
		return "", errorsx.Wrap(err, "unable to create the root container's archive")
	}
	defer func() { err = errorsx.Compact(err, tar.Close(), os.Remove(tar.Name())) }()

	if cid, err = execx.String(ctx, "podman", "create", "eg"); err != nil {
		return "", errorsx.Wrap(err, "unable to create the root container")
	}
	cid = strings.TrimSpace(cid)
	defer func() { err = errorsx.Compact(err, exec.Command("podman", "rm", cid).Run()) }()

	// an export holds only the filesystem, the environment a container starts its processes with is
	// the image's configuration.
	if environ, err = execx.String(ctx, "podman", "image", "inspect", "--format", "{{range .Config.Env}}{{println .}}{{end}}", "eg"); err != nil {
		return "", errorsx.Wrap(err, "unable to read the root container's environment")
	}
	if err = os.WriteFile(filepath.Join(imagedir, "environment"), []byte(environ), 0644); err != nil {
		return "", errorsx.Wrap(err, "unable to write the root container's environment")
	}
	defer os.Remove(filepath.Join(imagedir, "environment"))
	if err = exec.CommandContext(ctx, "podman", "cp", "--archive=false", filepath.Join(imagedir, "environment"), cid+":/etc/environment").Run(); err != nil {
		return "", errorsx.Wrap(err, "unable to place the root container's environment")
	}

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

	if err = os.WriteFile(filepath.Join(imagedir, "toc"), []byte(toc), 0600); err != nil {
		return "", errorsx.Wrap(err, "unable to record the egiso's table of contents")
	}

	for stale := range fsx.KeepNewestN(1, fsx.Find(cache, fsx.Levels(1))).Each(ctx) {
		errorsx.Log(errorsx.Wrapf(fsx.RemoveAll(stale), "egiso cache cleanup: %s", stale))
	}

	return location + "#" + toc.String(), nil
}

// environ is the environment a container of root starts its processes with: the image's, which
// root keeps as its /etc/environment, and root's home.
func environ(root fs.FS) (_ []string, err error) {
	var (
		image  []byte
		passwd []byte
		users  []user.User
		b      = envx.Build()
	)

	if image, err = fs.ReadFile(root, "etc/environment"); err != nil && fsx.ErrIsNotExist(err) == nil {
		return nil, errorsx.Wrap(err, "unable to read the root's environment")
	}

	if passwd, err = fs.ReadFile(root, "etc/passwd"); err != nil {
		return nil, errorsx.Wrap(err, "unable to read the root's accounts")
	}

	if users, err = user.ParsePasswdFilter(bytes.NewReader(passwd), func(u user.User) bool { return u.Uid == 0 }); err != nil {
		return nil, errorsx.Wrap(err, "unable to parse the root's accounts")
	}

	b = b.FromReader(bytes.NewReader(image)).Var("container", "egiso") // podman's containers set it to podman, the egiso sets its own.
	for _, u := range users {
		b = b.Var("HOME", u.Home)
	}

	return b.Environ()
}

// Do runs the module in the machine, as c8sproxy.PodmanModule runs it in a container.
func (t *Machine) Do(ctx context.Context, m transpile.Compiled) (err error) {
	if err = iox.Copy(m.Path, filepath.Join(t.ws.RuntimeDir, eg.ModuleBin)); err != nil {
		return errorsx.Wrap(err, "unable to place the module")
	}
	return t.module(ctx, eg.ModuleMount())
}

// module runs the module at path within the machine with the environment of a container of the
// root, the workspace's environment file, the machine's own and the stdio of eg.
func (t *Machine) module(ctx context.Context, path string) (err error) {
	var (
		env       []string
		verbosity string
	)

	if x := envx.Int(0, eg.EnvComputeLoggingVerbosity); x > 0 {
		verbosity = fmt.Sprintf("-%s", strings.Repeat("v", x))
	}

	if env, err = envx.Build().FromEnviron(t.environ...).FromPath(filepath.Join(t.ws.RuntimeDir, eg.EnvironFile)).FromEnviron(t.moduleenv...).Environ(); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

	// TODO REVISIT using t.ws.RuntimeDir as moduledir, as containers do.
	return t.vm.Exec(ctx, &execproxy.ExecRequest{
		Cmd:         envx.String("eg", eg.EnvComputeBin),
		Arguments:   slicesx.Filter(stringsx.Present, verbosity, "module", "--directory", eg.DefaultWorkingDirectory(), "--moduledir", t.ws.RuntimeDir, path),
		Dir:         "/",
		Environment: env,
	}, egvm.ExecOptionStdin(os.Stdin), egvm.ExecOptionStdout(log.Writer()), egvm.ExecOptionStderr(log.Writer()))
}

// machines boots the machines a machine's modules ask for beside it, implementing c8s.Proxy's Module
// as a machine runs it: a request's image is the uri of the egiso it boots, the machine's own when
// blank, its module the path of the module in the workspace's build directory, and its options the
// environment the module runs with. A machine beside it boots as it did, sharing its workspace and
// container storage.
type machines struct {
	c8s.UnimplementedProxyServer
	c       *http.Client
	ws      workspaces.Context
	uri     string
	console io.Writer
	options []MachineOption
}

// Module implements c8s.ProxyServer.
func (t machines) Module(ctx context.Context, req *c8s.ModuleRequest) (_ *c8s.ModuleResponse, err error) {
	var (
		sibling *Machine
	)

	if sibling, err = NewMachine(ctx, t.c, t.ws, stringsx.First(req.Image, t.uri), t.console, append(slices.Clone(t.options), MachineOptionEnviron(req.Options...))...); err != nil {
		return nil, err
	}
	defer func() { err = errorsx.Compact(err, sibling.Close()) }()

	if err = sibling.module(ctx, eg.DefaultMountRoot(eg.RuntimeDirectory, req.Module)); err != nil {
		return nil, err
	}
	return &c8s.ModuleResponse{}, nil
}

// Close shuts the machine down and releases its egiso.
func (t *Machine) Close() error {
	var errs []error
	if t.siblings != nil {
		t.siblings.Stop()
	}
	if t.vm != nil {
		errs = append(errs, t.vm.Close())
	}
	if t.dir != "" {
		errs = append(errs, os.RemoveAll(t.dir))
	}
	if t.upper != nil {
		errs = append(errs, t.upper.Close())
	}
	if t.archive != nil {
		errs = append(errs, t.archive.Close())
	}
	return errorsx.Compact(errs...)
}
