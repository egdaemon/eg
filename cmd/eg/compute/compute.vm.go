package compute

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/secrets"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
	"github.com/gofrs/uuid/v5"
	"github.com/opencontainers/go-digest"
)

// vm runs a workload the way local does, in a machine booted from an egiso in place of the root
// container. The egiso is the machine's root, under a writable upper kept in the workspace; its own
// systemd boots the machine and mounts the workspace's directories where a container mounts them as
// volumes.
type vm struct {
	Dir              string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	Debug            bool     `name:"debug" help:"keep workspace around to debug issues, requires manual cleanup"`
	AutoPatch        bool     `name:"auto-patch" help:"apply the changes a workload made to its working directory to the current worktree once it completes, when disabled the changes are printed as a patch instead (dry run)" default:"true" negatable:""`
	InvalidateCache  bool     `name:"invalidate-cache" help:"removes workload build cache"`
	EnvironmentPaths []string `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	GitRemote        string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_head_reference}"`
	Secrets          []string `name:"secret" help:"List of secret URIs to use. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
	Location         string   `name:"egiso" help:"path or https URL of the egiso the machine boots" required:""`
	TOC              string   `name:"toc" help:"digest of the egiso's table of contents" required:""`
	Name             string   `arg:"" name:"module" help:"name of the workload to run, i.e. the folder name within workload directory" default:"" predictor:"eg.workload"`
}

func (t vm) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, hotswapbin *cmdopts.HotswapPath) (err error) {
	var (
		ws        workspaces.Context
		repo      *git.Repository
		uid       = uuid.Must(uuid.NewV7())
		toc       digest.Digest
		archive   *egiso.Archive
		upperdir  string
		upper     *os.Root
		machine   *egvm.VM
		environ   []string
		verbosity string
		users     []egiso.Account
		groups    []egiso.Account
	)

	if toc, err = digest.Parse(t.TOC); err != nil {
		return errorsx.Wrap(err, "invalid --toc")
	}

	if ws, err = workspaces.NewLocal(
		gctx.Context,
		uuid.Must(uuid.NewV7()),
		md5x.Digest(cmdopts.BuildInfoSafe()),
		t.Dir,
		t.Name,
		workspaces.OptionEnabled(workspaces.OptionInvalidateModuleCache, t.InvalidateCache),
	); err != nil {
		return errorsx.Wrap(err, "unable to setup workspace")
	}
	if !t.Debug {
		defer os.RemoveAll(ws.Root)
	} else {
		log.Println("debug enabled", ws.Root)
	}

	defer ws.Cleanup(context.Background())

	if repo, err = git.PlainOpen(ws.WorkingDir); err != nil {
		return errorsx.Wrapf(err, "unable to open git repository: %s", ws.WorkingDir)
	}

	canonicaluri := errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote))

	envb := envx.Build().
		Var(eg.EnvGithubToken, githubToken(gctx.Context, true, canonicaluri)).
		FromReader(secrets.NewReader(gctx.Context, t.Secrets...)).
		FromPath(t.EnvironmentPaths...).
		FromEnviron(envx.AutoEnviron(t.Environment...)...).
		FromEnviron(errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))...).
		Var(eg.EnvComputeOS, runtime.GOOS).
		Var(eg.EnvComputeArch, runtime.GOARCH).
		Var(eg.EnvComputeWorkloadDirectory, eg.DefaultRootDirectory()).
		Var(eg.EnvComputeWorkingDirectory, eg.DefaultWorkingDirectory()).
		Var(eg.EnvComputeCacheDirectory, eg.DefaultCacheDirectory()).
		Var(eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory()).
		Var(eg.EnvComputeWorkspaceDirectory, eg.DefaultWorkspaceDirectory()).
		Var(eg.EnvComputeRunID, uid.String()).
		Var(eg.EnvComputeWorkload, t.Name).
		Var(eg.EnvComputeWorkloadName, t.Name).
		Var(eg.EnvComputeLoggingVerbosity, strconv.Itoa(gctx.Verbosity)).
		Var(eg.EnvComputeBin, hotswapbin.String()).
		Var(eg.EnvUnsafeCacheID, ws.CachedID).
		Var(eg.EnvComputeInitiatedTimestamp, time.Now().UTC().Format(time.RFC3339Nano)).
		Var(eg.EnvUnsafeGitCloneEnabled, strconv.FormatBool(false)) // hack to disable cloning

	if err = envb.WriteTo(filepath.Join(ws.RuntimeDir, eg.EnvironFile)); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}
	if environ, err = envb.Environ(); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

	roots, err := transpile.Autodetect(transpile.New(eg.DefaultModuleDirectory(t.Dir), ws)).Run(gctx.Context)
	if err != nil {
		return errorsx.Wrap(err, "unable to transpile")
	}

	if err = compile.EnsureRequiredPackages(gctx.Context, filepath.Join(ws.Root, ws.TransDir)); err != nil {
		return err
	}

	modules, err := compile.FromTranspiled(gctx.Context, ws, roots...)
	if err != nil {
		return err
	}

	if len(modules) == 0 {
		return errors.New("no usable modules detected")
	}

	if err = wasix.WarmCacheDirectory(gctx.Context, filepath.Join(ws.Root, ws.BuildDir), ws.CacheDirWazero); err != nil {
		log.Println("unable to prewarm wasi directory cache", err)
	}

	if archive, err = egiso.Open(gctx.Context, tlsc.DefaultClient(), t.Location, toc); err != nil {
		return err
	}
	defer archive.Close()

	if upperdir, err = os.MkdirTemp(ws.Root, "vm.*"); err != nil {
		return errorsx.Wrap(err, "unable to create the machine's upper")
	}

	if upper, err = os.OpenRoot(upperdir); err != nil {
		return errorsx.Wrap(err, "unable to open the machine's upper")
	}
	defer upper.Close()

	if users, groups, err = archive.Accounts(); err != nil {
		return err
	}

	// local mounts the eg running the command at egbin; machines share directories, not files.
	if err = iox.Copy(errorsx.Must(exec.LookPath(os.Args[0])), filepath.Join(ws.RuntimeDir, eg.BinaryBin)); err != nil {
		return errorsx.Wrap(err, "unable to place egbin")
	}

	// the machine has the image's own accounts, so the image's files keep their owners.
	root := egiso.NewSysFS(egiso.NewUpper(egiso.Tree{}.Mount(".", archive, egiso.Identity(users), egiso.Identity(groups)), upper), egiso.Default, egiso.Default)

	if machine, err = egvm.Boot(egvm.New(
		root,
		egvm.OptionAccounts(users, groups),
		egvm.OptionMountReadWrite(ws.CacheDir, eg.DefaultMountRoot(eg.CacheDirectory)),
		egvm.OptionMountReadWrite(ws.RuntimeDir, eg.DefaultMountRoot(eg.RuntimeDirectory)),
		egvm.OptionMountReadWrite(ws.WorkingDir, eg.DefaultMountRoot(eg.WorkingDirectory)),
		egvm.OptionMountReadWrite(ws.WorkspaceDir, eg.DefaultMountRoot(eg.WorkspaceDirectory)),
		egvm.OptionMountReadOnly(filepath.Join(ws.Root, ws.BuildDir, ws.Module, eg.ModuleDir), eg.DefaultMountRoot(eg.RuntimeDirectory, ws.Module, eg.ModuleDir)),
	), log.Writer()); err != nil {
		return err
	}
	defer func() { err = errorsx.Compact(err, machine.Close()) }()

	if err = machine.Exec(gctx.Context, &execproxy.ExecRequest{Cmd: "systemctl", Arguments: []string{"is-system-running", "--wait"}, Dir: "/"}); err != nil {
		return errorsx.Wrap(err, "the machine did not finish booting")
	}

	if gctx.Verbosity > 0 {
		verbosity = fmt.Sprintf("-%s", strings.Repeat("v", gctx.Verbosity))
	}

	for _, m := range modules {
		if err = iox.Copy(m.Path, filepath.Join(ws.RuntimeDir, eg.ModuleBin)); err != nil {
			return errorsx.Wrap(err, "unable to place the module")
		}

		// TODO REVISIT using t.ws.RuntimeDir as moduledir, as local does.
		if err = machine.Exec(gctx.Context, &execproxy.ExecRequest{
			Cmd:         envx.String("eg", eg.EnvComputeBin),
			Arguments:   slicesx.Filter(stringsx.Present, verbosity, "module", "--directory", eg.DefaultWorkingDirectory(), "--moduledir", ws.RuntimeDir, eg.ModuleMount()),
			Dir:         "/",
			Environment: environ,
		}, egvm.ExecOptionStdin(os.Stdin), egvm.ExecOptionStdout(log.Writer()), egvm.ExecOptionStderr(log.Writer())); err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	patch, err := gitx.Changes(gctx.Context, ws.WorkingDir, t.Dir)
	if err != nil {
		return err
	}

	if !t.AutoPatch {
		_, err = os.Stdout.Write(patch)
		return err
	}

	return gitx.ApplyPatch(gctx.Context, t.Dir, patch)
}
