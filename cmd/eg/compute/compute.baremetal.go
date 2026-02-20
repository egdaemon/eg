package compute

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/grpcx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/runtimex"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/secrets"
	"github.com/egdaemon/eg/interp"
	"github.com/egdaemon/eg/interp/c8sproxy"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
	"github.com/gofrs/uuid/v5"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type baremetal struct {
	cmdopts.RuntimeResources
	Dir             string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	GitRemote       string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference    string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_head_reference}"`
	Environment     []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Secrets         []string `name:"secret" help:"List of secret URIs to use. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
	Clone           bool     `name:"git-clone" help:"allow cloning via git"`
	InvalidateCache bool     `name:"invalidate-cache" help:"removes workload build cache"`
	Infinite        bool     `name:"infinite" help:"allow this module to run forever, used for running a workload like a webserver" hidden:"true"`
	Podman          bool     `name:"podman" help:"enable/disable podman" hidden:"true" negatable:"" default:"true"`
	Workload        string   `arg:"" help:"name of the workload to run" default:"" predictor:"eg.workload"`
}

func (t baremetal) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, hotswapbin *cmdopts.HotswapPath) (err error) {
	var (
		ws         workspaces.Context
		repo       *git.Repository
		environio  *os.File
		aid        = envx.String(uuid.Nil.String(), eg.EnvComputeAccountID)
		uid        = uuid.Must(uuid.NewV7())
		descr      = envx.String("", eg.EnvComputeVCS)
		cc         grpc.ClientConnInterface
		cgroups                        = envx.Toggle(runners.AgentOptionNoop, runners.AgentOptionCommandLine("--cgroups", "disabled"), envx.Int(0, eg.EnvComputeModuleNestedLevel) > 0)       // disable cgroups
		hostnet                        = envx.Toggle(runners.AgentOptionCommandLine("--network", "host"), runners.AgentOptionNoop, envx.Boolean(false, eg.EnvExperimentalDisableHostNetwork)) // ipv4 group bullshit. pretty sure its a podman 4 issue that was resolved in podman 5. this is 'safe' to do because we are already in a container.
		mountegbin runners.AgentOption = runners.AgentOptionEGBin(
			envx.String(errorsx.Must(exec.LookPath(os.Args[0])), eg.EnvComputeBinAlt),
		)
		cmdenv []string
	)

	if t.Infinite {
		t.RuntimeResources.TTL = time.Duration(math.MaxInt)
	}

	// clean up the eg environment ensuring a clean starting state.
	resetenv := func() error {
		unsetlog := func(s string) error {
			debugx.Printf("baremetal unsetting envvar %s %s", s, stringsx.DefaultIfBlank(os.Getenv(s), "(empty)"))
			return os.Unsetenv(s)
		}

		return errorsx.Compact(
			unsetlog(eg.EnvComputeModuleSocket),
			unsetlog(eg.EnvComputeModuleNestedLevel),
		)
	}

	if err := resetenv(); err != nil {
		return err
	}

	ctx := gctx.Context
	if t.Podman {
		ctx, err = podmanx.WithClient(gctx.Context)
		if t.Podman && err != nil {
			return errorsx.Wrap(err, "unable to connect to podman")
		}
	}

	// ensure when we run modules our umask is set to allow git clones to work properly
	runtimex.Umask(0002)

	if ws, err = workspaces.NewLocal(
		ctx, uuid.Must(uuid.NewV7()),
		md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())),
		t.Dir,
		t.Workload,
		workspaces.OptionEnabled(workspaces.OptionInvalidateModuleCache, t.InvalidateCache),
	); err != nil {
		return err
	}

	defer os.RemoveAll(ws.Root)

	if repo, err = git.PlainOpen(ws.WorkingDir); err != nil {
		return errorsx.Wrapf(err, "unable to open git repository: %s", ws.WorkingDir)
	}

	roots, err := transpile.Autodetect(transpile.New(eg.DefaultModuleDirectory(t.Dir), ws)).Run(ctx)
	if err != nil {
		return err
	}

	if err = compile.EnsureRequiredPackages(ctx, filepath.Join(ws.Root, ws.TransDir)); err != nil {
		return err
	}

	modules, err := compile.FromTranspiled(ctx, ws, roots...)
	if err != nil {
		return err
	}

	if len(modules) == 0 {
		return errors.New("no usable modules detected")
	}

	debugx.Println("modules", modules)

	if err = wasix.WarmCacheDirectory(ctx, filepath.Join(ws.Root, ws.BuildDir), ws.CacheDirWazero); err != nil {
		log.Println("unable to prewarm wasi directory cache", err)
	}

	var (
		control   net.Listener
		db        *sql.DB
		vmemlimit int64
	)

	if vmemlimit, err = memlimit.SetGoMemLimitWithOpts(memlimit.WithProvider(memlimit.FromSystem)); err != nil {
		return errorsx.Wrap(err, "unable to set max limits")
	}

	debugx.Println("---------------------------- BAREMETAL INITIATED ----------------------------")
	debugx.Println("module pid", os.Getpid())
	debugx.Println("cwd", errorsx.Zero(os.Getwd()))
	debugx.Println("account", aid)
	debugx.Println("run id", uid)
	debugx.Println("repository", descr)
	debugx.Println("number of cores (GOMAXPROCS - inaccurate)", runtime.GOMAXPROCS(-1))
	debugx.Println("ram available", bytesx.Unit(vmemlimit))
	debugx.Println("logging level", gctx.Verbosity)
	debugx.Fn(func() {
		envx.Debug(os.Environ()...)
	})
	defer debugx.Println("---------------------------- BAREMETAL COMPLETED ----------------------------")
	errorsx.Never(fsx.DirExists(ws.RuntimeDir))
	cspath := filepath.Join(ws.RuntimeDir, eg.SocketControl)
	if control, err = net.Listen("unix", cspath); err != nil {
		return errorsx.Wrapf(err, "unable to create socket %s", cspath)
	}
	defer control.Close()

	if db, err = sql.Open("duckdb", filepath.Join(ws.RuntimeDir, "analytics.db")); err != nil {
		return errorsx.Wrap(err, "unable to create analytics.db")
	}
	defer db.Close()

	if err = events.PrepareDB(ctx, db); err != nil {
		return errorsx.Wrap(err, "unable to prepare analytics.db")
	}

	gitenv := errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))
	cmdenvb := envx.Build().
		FromReader(secrets.NewReader(ctx, t.Secrets...)).
		FromEnv(t.Environment...).
		FromEnv(
		"PATH",
		"TERM",
		"COLORTERM",
		"LANG",
		"CI",
		eg.EnvComputeRunID,
		eg.EnvComputeAccountID,
	).Var(
		eg.EnvComputeBin, hotswapbin.String(),
	).Var(
		eg.EnvExperimentalBaremetal, strconv.FormatBool(true), // temporary while we flesh out the needed changes
	).Var(
		eg.EnvUnsafeGitCloneEnabled, strconv.FormatBool(t.Clone), // hack to disable cloning
	).Var(
		eg.EnvComputeLoggingVerbosity, strconv.Itoa(gctx.Verbosity),
	).Var(
		eg.EnvComputeModuleNestedLevel, strconv.Itoa(0),
	).Var(
		eg.EnvComputeTTL, t.RuntimeResources.TTL.String(),
	).Var(
		eg.EnvComputeOS, runtime.GOOS,
	).Var(
		eg.EnvComputeArch, runtime.GOARCH,
	).FromEnviron(
		gitenv...,
	)

	environpath := filepath.Join(ws.RuntimeDir, eg.EnvironFile)
	if environio, err = os.Create(environpath); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

	modulesenv := envx.Build().FromEnviron(errorsx.Must(cmdenvb.Environ())...).Var(
		eg.EnvComputeWorkloadDirectory, eg.DefaultWorkloadDirectory(),
	).Var(
		eg.EnvComputeWorkingDirectory, eg.DefaultWorkingDirectory(),
	).Var(
		eg.EnvComputeCacheDirectory, eg.DefaultCacheDirectory(),
	).Var(
		eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory(),
	).Var(
		eg.EnvComputeWorkspaceDirectory, eg.DefaultWorkspaceDirectory(),
	)

	if err = modulesenv.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

	// tune bare metal environment.
	cmdenvb.Var(
		eg.EnvComputeWorkloadDirectory, ws.Root,
	).Var(
		eg.EnvComputeRuntimeDirectory, ws.RuntimeDir,
	).Var(
		eg.EnvComputeCacheDirectory, ws.CacheDir,
	).Var(
		eg.EnvComputeWorkspaceDirectory, ws.WorkspaceDir,
	).Var(
		eg.EnvComputeWorkingDirectory, ws.WorkingDir,
	).Var(
		eg.EnvComputeDefaultGroup, defaultgroup(),
	).FromEnviron(
		os.Environ()...,
	)

	if cmdenv, err = cmdenvb.Environ(); err != nil {
		return err
	}

	// periodic sampling of system metrics
	go runners.BackgroundSystemLoad(ctx, db)

	// final sample
	defer func() {
		fctx, done := context.WithTimeout(ctx, 10*time.Second)
		defer done()
		errorsx.Log(runners.SampleSystemLoad(fctx, db))
	}()

	srv := grpc.NewServer(
		grpc.Creds(insecure.NewCredentials()), // this is a local socket
		grpc.ChainUnaryInterceptor(
			envx.Toggle(grpcx.NoopUnaryInterceptor, podmanx.GrpcClient, t.Podman),
		),
	)
	defer srv.GracefulStop()

	events.NewServiceDispatch(db).Bind(srv)
	execproxy.NewExecProxy(
		t.Dir,
		errorsx.Must(
			envx.Build().
				FromEnviron(gitenv...).
				FromEnviron(os.Environ()...).Environ(),
		),
	).Bind(srv)

	canonicaluri := errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote))
	ragent := runners.NewRunner(
		ctx,
		ws,
		uid.String(),
		runners.AgentOptionEnvironFile(environpath), // ensure we pick up the environment file with the container.
		runners.AgentOptionLocalComputeCachingVolumes(canonicaluri),
		runners.AgentOptionEnv(eg.EnvComputeTLSInsecure, strconv.FormatBool(tlsc.Insecure)),
		runners.AgentOptionVolumes(
			runners.AgentMountReadWrite(ws.WorkingDir, eg.DefaultMountRoot(eg.WorkingDirectory)),
			runners.AgentMountReadWrite(ws.CacheDir, eg.DefaultMountRoot(eg.CacheDirectory)),
			runners.AgentMountReadWrite(ws.RuntimeDir, eg.DefaultMountRoot(eg.RuntimeDirectory)),
			runners.AgentMountReadWrite(ws.WorkspaceDir, eg.DefaultMountRoot(eg.WorkspaceDirectory)),
		),
		runners.AgentOptionHostOS(),
		cgroups,
		mountegbin,
		hostnet,
	)

	c8sproxy.NewServiceProxy(
		log.Default(),
		ws,
		c8sproxy.ServiceProxyOptionContainerOptions(
			ragent.Options()...,
		),
		c8sproxy.ServiceProxyOptionBaremetal,
	).Bind(srv)

	go func() {
		errorsx.Log(errorsx.Wrap(srv.Serve(control), "unable to serve control socket"))
	}()

	if cc, err = grpc.DialContext(ctx, fmt.Sprintf("unix://%s", cspath), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock()); err != nil {
		return errorsx.Wrap(err, "failed to dial control service")
	}

	for _, m := range modules {
		err := interp.Remote(
			ctx,
			ws,
			aid,
			uid.String(),
			cc,
			m.Path,
			interp.OptionEnviron(cmdenv...),
		)

		if err != nil {
			return errorsx.Wrap(err, "failed to run module")
		}
	}

	return nil
}
