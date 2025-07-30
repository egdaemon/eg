package compute

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
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
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/runtimex"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp"
	"github.com/egdaemon/eg/interp/c8sproxy"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/execproxy"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
	"github.com/gofrs/uuid"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type baremetal struct {
	Dir          string `name:"directory" help:"root directory of the repository" default:"${vars_git_directory}"`
	RuntimeDir   string `name:"runtimedir" help:"runtime directory" hidden:"true" default:"${vars_workload_directory}"`
	GitRemote    string `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference string `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_head_reference}"`
	Clone        bool   `name:"git-clone" help:"allow cloning via git"`
	Workload     string `arg:"" help:"name of the workload to run"`
}

func (t baremetal) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		ws         workspaces.Context
		repo       *git.Repository
		aid        = envx.String(uuid.Nil.String(), eg.EnvComputeAccountID)
		uid        = uuid.Must(uuid.NewV7())
		descr      = envx.String("", eg.EnvComputeVCS)
		cc         grpc.ClientConnInterface
		hostnet                        = envx.Toggle(runners.AgentOptionCommandLine("--network", "host"), runners.AgentOptionNoop, eg.EnvExperimentalDisableHostNetwork) // ipv4 group bullshit. pretty sure its a podman 4 issue that was resolved in podman 5. this is 'safe' to do because we are already in a container.
		mountegbin runners.AgentOption = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
		cmdenv     []string
	)

	// ctx, err := podmanx.WithClient(gctx.Context)
	// if err != nil {
	// 	return errorsx.Wrap(err, "unable to connect to podman")
	// }

	// ensure when we run modules our umask is set to allow git clones to work properly
	runtimex.Umask(0002)

	if ws, err = workspaces.New(gctx.Context, t.Dir, t.RuntimeDir, t.Workload); err != nil {
		return err
	}
	defer os.RemoveAll(filepath.Join(ws.Root, ws.RuntimeDir))

	if repo, err = git.PlainOpen(ws.Root); err != nil {
		return errorsx.Wrap(err, "unable to open git repository")
	}

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(gctx.Context)
	if err != nil {
		return err
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

	debugx.Println("modules", modules)

	if err = wasix.WarmCacheDirectory(gctx.Context, filepath.Join(ws.Root, ws.BuildDir), wasix.WazCacheDir(filepath.Join(ws.Root, ws.CacheDir, eg.DefaultModuleDirectory()))); err != nil {
		log.Println("unable to prewarm wasi directory cache", err)
	}

	cmdenvb := envx.Build().FromEnv(
		"PATH",
		"TERM",
		"COLORTERM",
		"LANG",
		"CI",
		eg.EnvComputeBin,
		eg.EnvComputeRunID,
		eg.EnvComputeAccountID,
	).Var(
		eg.EnvUnsafeGitCloneEnabled, strconv.FormatBool(t.Clone), // hack to disable cloning
	).Var(
		eg.EnvComputeWorkingDirectory, eg.DefaultWorkingDirectory(),
	).Var(
		eg.EnvComputeCacheDirectory, envx.String(eg.DefaultCacheDirectory(), eg.EnvComputeCacheDirectory, "CACHE_DIRECTORY"),
	).Var(
		eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory(),
	).Var(
		"PAGER", "cat", // no paging in this environmenet.
	).Var(
		eg.EnvComputeModuleNestedLevel, strconv.Itoa(0),
	).FromEnviron(errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))...)

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
	debugx.Println("account", aid)
	debugx.Println("run id", uid)
	debugx.Println("repository", descr)
	debugx.Println("number of cores (GOMAXPROCS - inaccurate)", runtime.GOMAXPROCS(-1))
	debugx.Println("ram available", bytesx.Unit(vmemlimit))
	debugx.Println("logging level", gctx.Verbosity)
	defer debugx.Println("---------------------------- BAREMETAL COMPLETED ----------------------------")

	cspath := filepath.Join(ws.Root, ws.RuntimeDir, eg.SocketControl)
	if control, err = net.Listen("unix", cspath); err != nil {
		return errorsx.Wrapf(err, "unable to create socket %s", cspath)
	}
	defer control.Close()

	if db, err = sql.Open("duckdb", filepath.Join(ws.Root, ws.RuntimeDir, "analytics.db")); err != nil {
		return errorsx.Wrap(err, "unable to create analytics.db")
	}
	defer db.Close()

	if err = events.PrepareDB(gctx.Context, db); err != nil {
		return errorsx.Wrap(err, "unable to prepare analytics.db")
	}

	cmdenvb = cmdenvb.Var(
		eg.EnvComputeModuleSocket, eg.DefaultMountRoot(eg.RuntimeDirectory, filepath.Base(cspath)),
	).FromEnviron(
		os.Environ()...,
	)
	if cmdenv, err = cmdenvb.Environ(); err != nil {
		return err
	}

	// periodic sampling of system metrics
	go runners.BackgroundSystemLoad(gctx.Context, db)

	// final sample
	defer func() {
		fctx, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		errorsx.Log(runners.SampleSystemLoad(fctx, db))
	}()
	srv := grpc.NewServer(
		grpc.Creds(insecure.NewCredentials()), // this is a local socket
		grpc.ChainUnaryInterceptor(
			podmanx.GrpcClient,
		),
	)
	defer srv.GracefulStop()

	events.NewServiceDispatch(db).Bind(srv)
	execproxy.NewExecProxy(t.Dir, cmdenv).Bind(srv)

	canonicaluri := errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote))
	ragent := runners.NewRunner(
		gctx.Context,
		ws,
		uid.String(),
		runners.AgentOptionLocalComputeCachingVolumes(canonicaluri),
		runners.AgentOptionEnvKeys(cmdenv...),
		runners.AgentOptionEnv(eg.EnvComputeTLSInsecure, strconv.FormatBool(tlsc.Insecure)),
		runners.AgentOptionVolumes(
			runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.CacheDir), eg.DefaultMountRoot(eg.CacheDirectory)),
			runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.RuntimeDir), eg.DefaultMountRoot(eg.RuntimeDirectory)),
		),
		runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0]))),
		runners.AgentOptionHostOS(),
		hostnet,
		mountegbin,
	)

	c8sproxy.NewServiceProxy(
		log.Default(),
		ws,
		c8sproxy.ServiceProxyOptionCommandEnviron(cmdenv...),
		c8sproxy.ServiceProxyOptionContainerOptions(
			ragent.Options()...,
		),
		c8sproxy.ServiceProxyOptionBaremetal,
	).Bind(srv)

	go func() {
		errorsx.Log(errorsx.Wrap(srv.Serve(control), "unable to serve control socket"))
	}()

	if cc, err = grpc.DialContext(gctx.Context, fmt.Sprintf("unix://%s", cspath), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock()); err != nil {
		return err
	}

	for _, m := range modules {
		err := interp.Remote(
			gctx.Context,
			aid,
			uid.String(),
			cc,
			ws.Root,
			m.Path,
			interp.OptionRuntimeDir(ws.RuntimeDir),
			interp.OptionEnviron(cmdenv...),
		)
		if err != nil {
			time.Sleep(time.Minute)
			return err
		}
	}

	return nil
}
