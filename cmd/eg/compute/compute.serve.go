package compute

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/internal/uuidx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/secrets"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
)

type serve struct {
	cmdopts.RuntimeResources
	Dir              string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	ModuleDir        string   `name:"moduledir" help:"must be a subdirectory in the provided directory" default:"${vars_workload_directory}"`
	InvalidateCache  bool     `name:"invalidate-cache" help:"removes workload build cache"`
	AutoPatch        bool     `name:"auto-patch" help:"commit the changes made inside the served workload to a new eg/ branch of the current repository once it stops, when disabled the changes are printed as a patch instead (dry run)" default:"true" negatable:""`
	Privileged       bool     `name:"privileged" help:"run the initial container in privileged mode"`
	Dirty            bool     `name:"dirty" help:"include user directories and environment variables" hidden:"true"`
	Wayland          bool     `name:"wayland" help:"bind-mount the host wayland display socket into the container"`
	GCPAuto          bool     `name:"gcp-auto" help:"use the default well known path for gcp's application default credentials"`
	GCP              string   `name:"gcp" help:"path to gcp's application default credentials"`
	EnvironmentPaths []string `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Secrets          []string `name:"secret" help:"List of secret URIs to use. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
	GitRemote        string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_head_reference}"`
	Ports            []int    `name:"ports" help:"list of ports to publish to the host system"`
	SSHKeyPath       string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	SSH              int      `name:"ssh" type:"sshport" default:"-1" help:"enable ssh access into the container as the egd user, using eg's own managed ssh identity; bare --ssh publishes on a free host port, --ssh=<port> on a fixed one"`
	Containerfile    string   `name:"containerfile" help:"containerfile to build the workload's container from, relative to the repository directory. by default the nearest Containerfile, from the workload's directory up to the module directory"`
	Name             string   `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:"" predictor:"eg.workload"`
}

func (t serve) datadir(rels ...string) string {
	return filepath.Join(t.Dir, t.ModuleDir, t.Name, filepath.Join(rels...))
}

// containerfile the Containerfile the workload's container is built from: the one asked for, otherwise the nearest
// one from the workload's directory up to the module directory, so a Containerfile in a directory serves the
// workloads below it and a workload's own takes precedence.
func (t serve) containerfile() (string, error) {
	if t.Containerfile != "" {
		path := t.Containerfile
		if !filepath.IsAbs(path) {
			path = filepath.Join(t.Dir, path)
		}

		if _, err := os.Stat(path); err != nil {
			return "", errorsx.Wrapf(err, "unable to use the containerfile %s", path)
		}

		return path, nil
	}

	var (
		root = filepath.Join(t.Dir, t.ModuleDir)
		dir  = t.datadir()
	)

	for {
		candidate := filepath.Join(dir, "Containerfile")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		if parent := filepath.Dir(dir); dir != root && parent != dir {
			dir = parent
			continue
		}

		return "", errorsx.Errorf("no Containerfile in %s or the directories above it up to %s", t.datadir(), root)
	}
}

func (t serve) Run(gctx *cmdopts.Global, hotswapbin *cmdopts.HotswapPath, session *cmdopts.ComputeSession, clidb *cmdopts.CLIDB) (err error) {
	var (
		uid       = uuidx.FirstNonNil(session.Session, uuidx.NewV7())
		ws        workspaces.Context
		repo      *git.Repository
		environio *os.File
		rt        runners.T
	)

	ctx, err := podmanx.WithClient(gctx.Context)
	if err != nil {
		return errorsx.Wrap(err, "unable to connect to podman")
	}

	if ws, err = workspaces.NewLocal(
		gctx.Context,
		uuidx.NewV7(),
		md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())),
		t.Dir,
		t.Name,
		workspaces.OptionEnabled(workspaces.OptionInvalidateModuleCache, t.InvalidateCache),
	); err != nil {
		return errorsx.Wrap(err, "unable to setup workspace")
	}
	defer os.RemoveAll(ws.Root)

	if repo, err = git.PlainOpen(ws.WorkingDir); err != nil {
		return errorsx.Wrapf(err, "unable to open git repository: %s", ws.WorkingDir)
	}

	log.Println("loading environment file", t.datadir(".eg.env"))
	envb := envx.Build().
		FromEnv(os.Environ()...).
		FromReader(secrets.NewReader(gctx.Context, t.Secrets...)).
		FromPath(t.EnvironmentPaths...).
		FromPath(t.datadir(".eg.env")).
		FromEnv(t.Environment...).
		FromEnviron(errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))...).
		Var(eg.EnvComputeRunID, uid.String()).
		Var(eg.EnvComputeWorkloadName, t.Name).
		Var(eg.EnvComputeLoggingVerbosity, strconv.Itoa(gctx.Verbosity)).
		Var(eg.EnvComputeBin, hotswapbin.String()).
		Var(eg.EnvUnsafeCacheID, ws.CachedID).
		Var(eg.EnvComputeTTL, t.RuntimeResources.TTL.String()).
		Var(eg.EnvComputeInitiatedTimestamp, time.Now().UTC().Format(time.RFC3339Nano)).
		Var(eg.EnvUnsafeGitCloneEnabled, strconv.FormatBool(false)) // hack to disable cloning

	for idx, p := range t.Ports {
		envb = envb.Var(fmt.Sprintf("EG_COMPUTE_PORT_%d", idx), strconv.Itoa(p))
	}

	log.Println("cacheid", ws.CachedID)

	roots, err := transpile.Autodetect(transpile.New(eg.DefaultModuleDirectory(t.Dir), ws)).Run(gctx.Context)
	if err != nil {
		return err
	}

	if err = compile.EnsureRequiredPackages(gctx.Context, filepath.Join(ws.Root, ws.TransDir)); err != nil {
		return err
	}

	module, err := compile.OneFromTranspiled(gctx.Context, ws, roots...)
	if err != nil {
		return err
	}

	debugx.Println("module", module)

	debugx.Println("runtime resources", spew.Sdump(t.RuntimeResources))

	canonicaluri := errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote))
	if rt, err = t.podman(ctx, ws, uid, canonicaluri, envb); err != nil {
		return err
	}
	defer func() { err = errorsx.Compact(err, rt.Close()) }()

	if environio, err = os.Create(filepath.Join(ws.RuntimeDir, eg.EnvironFile)); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

	// the cli's database is optional, serve does not otherwise depend on it. registering does nothing without it.
	defer func() { errorsx.Log(clidb.Close()) }()

	// registered before the module starts so the container's ssh port is waited for as soon as it exists.
	registered, done := session.New(clidb).Branch(session.Branch, defaultbranch(t.Name, uid)).SSH(t.SSH, t.SSHKeyPath).Agent(true).Patch(t.AutoPatch).Register(ctx, uid, t.Name, t.Dir)
	defer func() { done(err) }()

	err = errorsx.Wrap(rt.Do(gctx.Context, module), "module execution failed")

	log.Println("--------------------------------------------------------------------------")
	log.Println("autopatch", t.AutoPatch)
	log.Println("branch", registered.Branch)
	log.Println("from", ws.WorkingDir)
	log.Println("to", t.Dir)
	// a served workload ends when its container is stopped, which is not a failure of the work done inside it,
	// so the changes are collected however the modules ended. the run's context is likely cancelled by then.
	if perr := gitx.AutoPatch(os.Stdout).From(ws.WorkingDir).To(t.Dir).Branch(registered.Branch).DryRun(!t.AutoPatch).Apply(context.WithoutCancel(gctx.Context)); perr != nil {
		log.Println("unable to apply the workload's changes to the worktree", perr)
		err = errorsx.Compact(err, perr)
	}
	log.Println("--------------------------------------------------------------------------")

	return err
}

// podman prepares the container the served module runs in, built from the workload's Containerfile.
func (t serve) podman(ctx context.Context, ws workspaces.Context, uid uuidx.UUID, canonicaluri string, envb *envx.Builder) (_ runners.T, err error) {
	var (
		homedir     = userx.HomeDirectoryOrDefault("/root")
		environpath = filepath.Join(ws.RuntimeDir, eg.EnvironFile)
		gnupghome   runners.AgentOption
		envvar      runners.AgentOption = runners.AgentOptionNoop
		mounthome   runners.AgentOption = runners.AgentOptionNoop
		wayland     runners.AgentOption = runners.AgentOptionNoop
		gcpcreds    runners.AgentOption = runners.AgentOptionNoop
		privileged  runners.AgentOption = runners.AgentOptionNoop
		mountegbin  runners.AgentOption = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
	)

	if t.Dirty {
		mounthome = runners.AgentOptionAutoMountHome(homedir)
	}

	if t.GCPAuto {
		gcpcreds = runners.AgentOptionGcloudCredentials(ctx, envb, envx.String(userx.DefaultConfigDirectory("gcloud", "application_default_credentials.json"), eg.EnvGoogleApplicationCredentials))
	} else if stringsx.Present(t.GCP) {
		gcpcreds = runners.AgentOptionGcloudCredentials(ctx, envb, t.GCP)
	}

	gnupghome = runners.AgentOptionLocalGPGAgent(ctx, envb)

	if t.Wayland {
		wayland = runners.AgentOptionWayland(ctx, envb)
	}

	sshserver := runners.AgentOptionSSHServer(ctx, ws, t.SSHKeyPath, t.SSH)

	imagename := stringsx.Join(".", "eg.serve", strings.ReplaceAll(t.Name, string(filepath.Separator), "."))
	containerfile, err := t.containerfile()
	if err != nil {
		return nil, errorsx.Wrap(err, "serve requires a containerfile to run")
	}

	if err = runners.BuildContainer(ctx, imagename, t.Dir, containerfile); err != nil {
		return nil, errorsx.Wrap(err, "serve requires a containerfile to run")
	}

	if err = wasix.WarmCacheDirectory(ctx, filepath.Join(ws.Root, ws.BuildDir), ws.CacheDirWazero); err != nil {
		log.Println("unable to prewarm wasi directory cache", err)
	}

	if t.Privileged {
		privileged = runners.AgentOptionCommandLine("--privileged")
	}

	ragent := runners.NewRunner(
		ctx,
		ws,
		uid.String(),
		privileged,
		mounthome,
		envvar,
		sshserver,
		mountegbin,
		runners.AgentOptionVolumes(
			runners.AgentMountReadWrite(ws.CacheDir, eg.DefaultMountRoot(eg.CacheDirectory)),
			runners.AgentMountReadWrite(ws.RuntimeDir, eg.DefaultMountRoot(eg.RuntimeDirectory)),
			runners.AgentMountReadWrite(ws.WorkingDir, eg.DefaultMountRoot(eg.WorkingDirectory)),
			runners.AgentMountReadWrite(ws.WorkspaceDir, eg.DefaultMountRoot(eg.WorkspaceDirectory)),
		),
		runners.AgentOptionLocalComputeCachingVolumes(canonicaluri),
		gnupghome, // must come after the runtime directory mount to ensure correct mounting order.
		gcpcreds,  // must come after the mount directory to ensure correct mounting order.
		wayland,   // must come after the runtime directory mount to ensure correct mounting order.
		runners.AgentOptionEnvironFile(environpath), // ensure we pick up the environment file with the container.
		runners.AgentOptionHostOS(),
		runners.AgentOptionPublish(t.Ports...),
		runners.AgentOptionCores(t.RuntimeResources.Cores),
		runners.AgentOptionMemory(uint64(t.RuntimeResources.Memory)),
	)

	return container{ctx: ctx, ws: ws, uid: uid, agent: ragent, image: imagename}, nil
}
