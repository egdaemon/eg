package compute

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/slicesx"
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

// bootstrapWorkload is the name of the builtin workload that builds eg and runs workloads with it.
const bootstrapWorkload = "bootstrapped"

// builtinBootstrap is `compute local` run with an eg built from the repository instead of the one installed. it takes the
// flags of local so a command line works with either by swapping the command, e.g.)
//
//	eg compute local --secret gcpsm://project/name -vv release/egcoreutils
//	eg compute builtin bootstrap --secret gcpsm://project/name -vv release/egcoreutils
//
// like serve it runs the module in a container built from the repository's .eg/Containerfile. the module, the builtin
// workload bootstrapped, builds eg and runs `eg --hotswap compute local` with the arguments it is given, the command line
// of this command after bootstrap less the flags this command applies itself, see hostflags. secrets are read here, the
// host has the credentials for them, and are in the environment of the container, which the inner command inherits.
type builtinBootstrap struct {
	cmdopts.RuntimeResources
	cmdopts.Publishing
	Dir              string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	ModuleDir        string   `name:"moduledir" help:"must be a subdirectory in the provided directory" default:"${vars_workload_directory}" hidden:"true"`
	Debug            bool     `name:"debug" help:"keep workspace around to debug issues, requires manual cleanup"`
	AutoPatch        bool     `name:"auto-patch" help:"commit the changes a workload made to its working directory to a new eg/ branch of the current repository once it completes, when disabled the changes are printed as a patch instead (dry run)" default:"true" negatable:""`
	Privileged       bool     `name:"privileged" help:"run the initial container in privileged mode"`
	Cgroups          bool     `name:"cgroups" help:"enable/disable cgroups for the module container" hidden:"true" negatable:"" default:"true"`
	Dirty            bool     `name:"dirty" help:"include user directories and environment variables" hidden:"true"`
	Wayland          bool     `name:"wayland" help:"bind-mount the host wayland display socket into the container"`
	GPU              bool     `name:"gpu" help:"enable gpu support, implied by a non zero --vram" hidden:"true"`
	GithubAuto       bool     `name:"github-token" help:"automatically detect and inject github credentials" negatable:"" default:"true" env:"${env_github_autotoken}"`
	GCPAuto          bool     `name:"gcp-auto" help:"use the default well known path for gcp's application default credentials"`
	GCP              string   `name:"gcp" help:"path to gcp's application default credentials"`
	InvalidateCache  bool     `name:"invalidate-cache" help:"removes workload build cache"`
	EnvironmentPaths []string `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	GitRemote        string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_head_reference}"`
	Ports            []int    `name:"ports" help:"list of ports to publish to the host system" hidden:"true"`
	ContainerArgs    []string `name:"cargs" help:"list of command line arguments to pass to the root container" hidden:"true"`
	Secrets          []string `name:"secret" help:"List of secret URIs to use. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
	Profile          string   `name:"profile" help:"enable profiling of module runs (cpu,heap,mem,allocs,block)" enum:"cpu,heap,mem,allocs,block," default:""`
	Egiso            *string  `name:"egiso" help:"EXPERIMENTAL: run in a machine booted from the egiso at this path or https URL, with the digest of its table of contents as its fragment, built from the default root container when empty" hidden:"true"`
	Name             string   `arg:"" name:"module" help:"name of the workload to run, i.e. the folder name within workload directory" default:"" predictor:"eg.workload"`
}

// hostflags are the flags of compute local the bootstrap applies itself, to the container it runs the module in. the inner
// compute local must not get them: it would apply them again, to a container of its own. the value is whether the flag takes one.
var hostflags = map[string]bool{
	"--secret":     true, // read here, the host has the credentials for them, see Run.
	"--gcp":        true,
	"--gcp-auto":   false,
	"--privileged": false,
	"--dirty":      false,
	"--wayland":    false,
	"--ports":      true,
	"--cores":      true,
	"--memory":     true,
}

// bootstrapArgsEnv holds the arguments of the module, see encodeargs. the workload reads it, it must match the one in .builtin/bootstrapped.
const bootstrapArgsEnv = "EG_BOOTSTRAP_ARGS"

// encodeargs encodes args as a base64 json array.
func encodeargs(args []string) string {
	return base64.StdEncoding.EncodeToString(errorsx.Must(json.Marshal(args)))
}

// moduleargs are the arguments of the module, those of compute local that follow bootstrap in the command line args.
func moduleargs(args []string) []string {
	skip := false
	return slicesx.Filter(func(arg string) bool {
		if skip {
			skip = false
			return false
		}

		name, _, inline := strings.Cut(arg, "=")
		takesvalue, host := hostflags[name]
		skip = host && takesvalue && !inline

		return !host
	}, args[slices.Index(args, "bootstrap")+1:]...)
}

func (t builtinBootstrap) Run(gctx *cmdopts.Global, hotswapbin *cmdopts.HotswapPath) (err error) {
	var (
		tmpdir     string
		ws         workspaces.Context
		repo       *git.Repository
		environio  *os.File
		uid                            = uuidx.NewV7()
		mounthome  runners.AgentOption = runners.AgentOptionNoop
		gcpcreds   runners.AgentOption = runners.AgentOptionNoop
		privileged runners.AgentOption = runners.AgentOptionNoop
		mountegbin runners.AgentOption = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
	)

	ctx, err := podmanx.WithClient(gctx.Context)
	if err != nil {
		return errorsx.Wrap(err, "unable to connect to podman")
	}

	if tmpdir, err = os.MkdirTemp("", "eg.bootstrap.*"); err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}
	defer func() {
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	if ws, err = workspaces.NewBuiltin(
		gctx.Context,
		uuidx.NewV7(),
		md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())),
		tmpdir,
		bootstrapWorkload,
		workspaces.OptionSymlinkCache(filepath.Join(t.Dir, eg.CacheDirectory)),
		workspaces.OptionSymlinkWorking(t.Dir),
		workspaces.OptionEnabled(workspaces.OptionInvalidateModuleCache, t.InvalidateCache),
	); err != nil {
		return errorsx.Wrap(err, "unable to initialize workspace")
	}

	if err = fsx.CloneTree(gctx.Context, eg.DefaultModuleDirectory(ws.Root), ".builtin", embeddedbuiltin); err != nil {
		return errorsx.Wrap(err, "unable to clone tree")
	}

	if err = compile.InitGolang(gctx.Context, eg.DefaultModuleDirectory(ws.Root), cmdopts.ModPath()); err != nil {
		return errorsx.Wrap(err, "failed to init go mod")
	}

	if err = compile.InitGolangTidy(gctx.Context, eg.DefaultModuleDirectory(ws.Root)); err != nil {
		return errorsx.Wrap(err, "failed to tidy packages")
	}

	roots, err := transpile.Autodetect(transpile.New(eg.DefaultModuleDirectory(ws.Root), ws)).Run(gctx.Context)
	if err != nil {
		return err
	}

	module, err := compile.OneFromTranspiled(gctx.Context, ws, roots...)
	if err != nil {
		return errorsx.Wrap(err, "unable to transpile")
	}

	if err = compile.EnsureRequiredPackages(gctx.Context, filepath.Join(ws.Root, ws.TransDir)); err != nil {
		return errorsx.Wrap(err, "transpiled failed to ensure required packages")
	}

	if err = compile.InitGolangTidy(gctx.Context, filepath.Join(ws.Root, ws.TransDir)); err != nil {
		return errorsx.Wrap(err, "transpiled failed to tidy packages")
	}

	// the container is the repository's own, the one its workloads are developed in.
	imagename := "eg.bootstrap"
	if err = runners.BuildContainer(ctx, imagename, t.Dir, filepath.Join(t.Dir, t.ModuleDir, "Containerfile")); err != nil {
		return errorsx.Wrap(err, "bootstrap requires the repository's .eg/Containerfile")
	}

	if repo, err = git.PlainOpen(t.Dir); err != nil {
		return errorsx.Wrap(err, "unable to open git repository")
	}

	environpath := filepath.Join(ws.RuntimeDir, eg.EnvironFile)
	if environio, err = os.Create(environpath); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

	canonicaluri := errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote))

	envb := envx.Build().
		FromEnv(os.Environ()...).
		Var(eg.EnvGithubToken, githubToken(gctx.Context, t.GithubAuto, canonicaluri)).
		FromReader(secrets.NewReader(gctx.Context, t.Secrets...)).
		FromPath(t.EnvironmentPaths...).
		FromEnv(t.Environment...).
		FromEnviron(errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))...).
		Var(eg.EnvComputeRunID, uid.String()).
		Var(eg.EnvComputeWorkloadName, bootstrapWorkload).
		Var(eg.EnvComputeLoggingVerbosity, strconv.Itoa(gctx.Verbosity)).
		Var(eg.EnvComputeBin, hotswapbin.String()).
		Var(bootstrapArgsEnv, encodeargs(moduleargs(os.Args))).
		Var(eg.EnvUnsafeCacheID, ws.CachedID).
		Var(eg.EnvComputeTTL, t.RuntimeResources.TTL.String()).
		Var(eg.EnvUnsafeGitCloneEnabled, strconv.FormatBool(false)) // hack to disable cloning

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

	if err = wasix.WarmCacheDirectory(gctx.Context, filepath.Join(ws.Root, ws.BuildDir), ws.CacheDirWazero); err != nil {
		log.Println("unable to prewarm wasi directory cache", err)
	}

	if t.Dirty {
		mounthome = runners.AgentOptionAutoMountHome(userx.HomeDirectoryOrDefault("/root"))
	}

	if t.GCPAuto {
		gcpcreds = runners.AgentOptionGcloudCredentials(ctx, envb, envx.String(userx.DefaultConfigDirectory("gcloud", "application_default_credentials.json"), eg.EnvGoogleApplicationCredentials))
	} else if stringsx.Present(t.GCP) {
		gcpcreds = runners.AgentOptionGcloudCredentials(ctx, envb, t.GCP)
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
		mountegbin,
		runners.AgentOptionVolumes(
			runners.AgentMountReadWrite(ws.CacheDir, eg.DefaultMountRoot(eg.CacheDirectory)),
			runners.AgentMountReadWrite(ws.RuntimeDir, eg.DefaultMountRoot(eg.RuntimeDirectory)),
			runners.AgentMountReadWrite(ws.WorkingDir, eg.DefaultMountRoot(eg.WorkingDirectory)),
			runners.AgentMountReadWrite(ws.WorkspaceDir, eg.DefaultMountRoot(eg.WorkspaceDirectory)),
		),
		runners.AgentOptionLocalComputeCachingVolumes(canonicaluri),
		gcpcreds, // must come after the mount directory to ensure correct mounting order.
		runners.AgentOptionEnvironFile(environpath),
		runners.AgentOptionCommandLine("--env-file", environpath), // required for tty to work correctly in local mode.
		runners.AgentOptionHostOS(),
		runners.AgentOptionPublish(t.Ports...),
		runners.AgentOptionCores(t.RuntimeResources.Cores),
		runners.AgentOptionMemory(uint64(t.RuntimeResources.Memory)),
		runners.AgentOptionEnv(eg.EnvComputeRunID, uid.String()),
		runners.AgentOptionEnv(eg.EnvComputeWorkloadName, bootstrapWorkload),
		runners.AgentOptionEnv(eg.EnvComputeLoggingVerbosity, strconv.Itoa(gctx.Verbosity)),
	)

	return errorsx.Wrap(container{
		ctx:   ctx,
		ws:    ws,
		uid:   uid,
		agent: ragent,
		image: imagename,
	}.Do(gctx.Context, module), "module execution failed")
}
