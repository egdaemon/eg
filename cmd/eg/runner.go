package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/duckproxy/duckgres"
	"github.com/egdaemon/duckproxy/duckserve"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/gcpx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/memlimitx"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/runtimex"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/ttyx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/internal/uuidx"
	"github.com/egdaemon/eg/interp"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/c8sproxy"
	"github.com/egdaemon/eg/interp/dbproxy/dbservice"
	"github.com/egdaemon/eg/interp/environ"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/execproxy/v2"
	"github.com/egdaemon/eg/interp/machines"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/egworkloads"
	"github.com/egdaemon/eg/workspaces"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/egdaemon/gdx"
	"go.uber.org/automaxprocs/maxprocs"

	_ "github.com/radovskyb/watcher"
)

type module struct {
	Dir        string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	ModuleDir  string   `name:"moduledir" help:"deprecated removed once infrastructure is updated" hidden:"true" default:"${vars_workload_directory}"`
	RuntimeDir string   `name:"runtimedir" help:"runtime directory" hidden:"true" default:"${vars_eg_runtime_directory}"`
	Isolate    string   `name:"isolate" help:"path of the Containerfile to build and run the module within, instead of within this container" hidden:"true" env:"${env_eg_compute_module_isolate}"`
	Module     string   `arg:"" help:"name of the module to run"`
	Args       []string `arg:"" optional:"" help:"arguments for the module, its os.Args[1:]"`
}

// remapdir returns a function that remaps a directory, giving the unprivileged user access to the contents.
// A machine's owned directory is bound as it is: every user already has access, and bindfs would only
// add a mount to go through.
func remapdir(ctx context.Context, bindfs string) func(from, to string) error {
	return func(from, to string) (err error) {
		if egvm.Owned(from) {
			mcmd := exec.CommandContext(ctx, "mount", "--bind", from, to)
			mcmd.Stdout, mcmd.Stderr = log.Writer(), log.Writer()
			return errorsx.Wrapf(execx.MaybeRun(mcmd), "unable to bind: %s", from)
		}

		if envx.Boolean(false, eg.EnvExperimentalBindMount) {
			egdu, err := user.Lookup(eg.DefaultUsername)
			if err != nil {
				return errorsx.Wrap(err, "failed to lookup user egd")
			}

			log.Println("binding mount", egdu.Name, egdu.Uid, egdu.Gid)
			mcmd := exec.CommandContext(
				ctx,
				"mount",
				"--bind",
				fmt.Sprintf("--map-users=0:%s:1", egdu.Uid),
				fmt.Sprintf("--map-groups=0:%s:1", egdu.Gid),
				from,
				to,
			)
			mcmd.Stdout, mcmd.Stderr = log.Writer(), log.Writer()
			if err = execx.MaybeRun(mcmd); err != nil {
				return errorsx.Wrapf(err, "unable to run bindfs: %s", from)
			}
		} else {
			mcmd := exec.CommandContext(ctx, bindfs, "--map=root/egd:@root/@egd", "-o", "entry_timeout=0", from, to)
			mcmd.Stdout, mcmd.Stderr = log.Writer(), log.Writer()
			if err = execx.MaybeRun(mcmd); err != nil {
				return errorsx.Wrapf(err, "unable to run bindfs: %s", from)
			}
			// watcherx proxy was too expensive.
			// go func() {
			// 	errorsx.Log(
			// 		errorsx.Wrapf(
			// 			watcherx.Proxy(ctx, from, to, 10*time.Millisecond),
			// 			"watcher proxy failed for %s -> %s", from, to,
			// 		),
			// 	)
			// }()
		}

		return nil
	}
}

// defaultgcloudcredentials points gcloud at the credentials installed for the workload, gcloud commands only use them when told to.
// the environment of the process is used by the module and the commands it runs. a value already in the environment is
// the users override and is kept. it must not be set when there are no credentials, it would shadow the user's own gcloud login.
func defaultgcloudcredentials(adc string) error {
	if adc == "" {
		return nil
	}

	if _, set := os.LookupEnv(eg.EnvCloudSDKCredentialFileOverride); set {
		return nil
	}

	return os.Setenv(eg.EnvCloudSDKCredentialFileOverride, adc)
}

// installcredentials installs the gcp credentials provided to the workload, see gcpx.InstallCredentials.
func installcredentials(ctx context.Context) error {
	adc, err := gcpx.InstallCredentials(ctx, eg.DefaultRuntimeDirectory(gcpx.Directory))
	if err != nil {
		return err
	}

	return defaultgcloudcredentials(adc)
}

func (t module) mounthack(ctx context.Context, runid string, ws workspaces.Context) (err error) {
	var (
		binname = "bindfs"
		mbin    string
	)

	if mbin, err = exec.LookPath(binname); err != nil {
		return errorsx.Wrap(err, "unable to locate bindfs, failing")
	}

	remap := remapdir(ctx, mbin)

	err = fsx.MkDirs(
		0770,
		eg.DefaultWorkingDirectory(),
		eg.DefaultCacheDirectory(),
		eg.DefaultWorkspaceDirectory(),
	)
	if err != nil {
		return err
	}

	err = errors.Join(
		remap(eg.DefaultMountRoot(eg.WorkingDirectory), eg.DefaultWorkingDirectory()),
		remap(eg.DefaultMountRoot(eg.CacheDirectory), eg.DefaultCacheDirectory()),
		remap(eg.DefaultMountRoot(eg.WorkspaceDirectory), eg.DefaultWorkspaceDirectory()),
	)
	if err != nil {
		return err
	}

	if dirstr := envx.String("", eg.EnvUnsafeRemapDirectory); stringsx.Present(dirstr) {
		dirs := strings.Split(dirstr, ":")
		debugx.Println("remap dirs", dirs)
		err = fsx.MkDirs(
			0770,
			dirs...,
		)
		if err != nil {
			log.Println("failed to mkdirs for remap", err)
			// 	return err
		}

		for _, d := range dirs {
			rel, err := filepath.Rel(eg.DefaultRootDirectory(), d)
			if err != nil {
				log.Println("unable to map directory", d, err)
				continue
			}

			debugx.Println("remapping", eg.DefaultMountRoot(rel), "->", d)
			if err := remap(eg.DefaultMountRoot(rel), d); err != nil {
				log.Println("map failed", d, err)
				continue
				// return err
			}
		}
	}

	// HACK: gpg no longer obeys GNUPGHOME for the root user.
	if err = errorsx.Compact(fsx.MkDirs(0700, "/run/user/0/gnupg"), os.Symlink("/eg.mnt/.gnupg", "/run/user/0/gnupg/d.3uqafziaitu1mwy9asijecpi")); err != nil {
		return err
	}

	if err = fsx.Wait(ctx, 3*time.Second, ws.Root); err != nil {
		return errorsx.Wrapf(err, "expected working directory (%s) did not appear, this is a known issue", ws.Root)
	}

	return nil
}

// bridges the host wayland compositor socket (bind-mounted read-write, but
// root-owned from the unprivileged egd user's perspective, by
// runners.AgentOptionWayland) to an egd-owned socket via waypipe, since
// bindfs can't proxy live sockets the way mounthack remaps directories.
// waypipe has no uid/gid option, so the control socket it creates as root
// is chmod'd after the fact so the unprivileged server side can reach it.
func (t module) waylandhack(ctx context.Context) (err error) {
	hostsock := envx.String("", "WAYLAND_HOST_SOCKET")
	if hostsock == "" {
		return nil // --wayland not requested, AgentOptionWayland never set this.
	}

	ctrlsock := envx.String("/tmp/waypipe.sock", "WAYPIPE_CTRL_SOCKET")
	display := envx.String("/tmp/wayland.sock", "WAYLAND_DISPLAY")

	wbin, err := exec.LookPath("waypipe")
	if err != nil {
		return errorsx.Wrap(err, "unable to locate waypipe")
	}

	// client: this process is already root; it creates ctrlsock and connects
	// out to the real (root-owned, bind-mounted) host compositor socket.
	clientcmd := exec.CommandContext(ctx, wbin, "-s", ctrlsock, "client")
	clientcmd.Env = append(os.Environ(), "WAYLAND_DISPLAY="+hostsock)
	clientcmd.Stdout, clientcmd.Stderr = log.Writer(), log.Writer()
	if err = clientcmd.Start(); err != nil {
		return errorsx.Wrap(err, "unable to start waypipe client")
	}
	go func() {
		errorsx.Log(errorsx.Wrap(clientcmd.Wait(), "waypipe client exited"))
	}()

	if err = fsx.Wait(ctx, 3*time.Second, ctrlsock); err != nil {
		return errorsx.Wrapf(err, "waypipe control socket did not appear: %s", ctrlsock)
	}

	if err = os.Chmod(ctrlsock, 0666); err != nil {
		return errorsx.Wrapf(err, "unable to chmod waypipe control socket: %s", ctrlsock)
	}

	// server: unprivileged (egd), connects to ctrlsock, creates the
	// app-facing display socket that pipeline steps connect to normally.
	servercmd := exec.CommandContext(ctx, "sudo", "-u", eg.DefaultUsername, "-g", eg.DefaultUsername, wbin, "-s", ctrlsock, "--display", display, "server", "--", "sleep", "infinity")
	servercmd.Stdout, servercmd.Stderr = log.Writer(), log.Writer()
	if err = servercmd.Start(); err != nil {
		return errorsx.Wrap(err, "unable to start waypipe server")
	}
	go func() {
		errorsx.Log(errorsx.Wrap(servercmd.Wait(), "waypipe server exited"))
	}()

	return fsx.Wait(ctx, 3*time.Second, display)
}

// recordDiscoveredWorkloads scans dir for eg workloads and records what it
// finds into db, but only for default-branch builds (see
// eg.IsDefaultBranchBuild) -- everything else (PR builds, feature branches)
// is skipped. failures are logged, not fatal: this is analytics bookkeeping
// and shouldn't fail an actual workload run.
func recordDiscoveredWorkloads(ctx context.Context, db *sql.DB, dir string, environ ...string) {
	if !eg.IsDefaultBranchBuild(environ...) {
		return
	}

	if err := egworkloads.Detect(ctx, db, dir); err != nil {
		log.Println("unable to record discovered workloads", err)
	}
}

// isolated builds the workload's Containerfile and runs the module within the resulting container, instead of within
// this one. both go through the container proxy of the root module, the same as a module that builds and runs a container.
func (t module) isolated(ctx context.Context, cc grpc.ClientConnInterface, uid, containerfile string) (err error) {
	var (
		containers = c8s.NewProxyClient(cc)
		image      = stringsx.Join(".", eg.WorkingDirectory, strings.ReplaceAll(envx.String("workload", eg.EnvComputeWorkloadName), string(filepath.Separator), "."))
	)

	log.Println("building the workload's container", image, containerfile)
	if _, err = containers.Build(ctx, &c8s.BuildRequest{Name: image, Directory: t.Dir, Definition: containerfile}); err != nil {
		return errorsx.Wrap(err, "unable to build the workload's containerfile")
	}

	_, err = containers.Module(ctx, &c8s.ModuleRequest{
		Image:  image,
		Name:   fmt.Sprintf("eg-workload-%s", uid),
		Module: filepath.Base(t.Module),
		Mdir:   t.RuntimeDir,
		Options: []string{
			"--env", fmt.Sprintf("%s=%d", eg.EnvComputeModuleNestedLevel, envx.Int(0, eg.EnvComputeModuleNestedLevel)+1),
		},
	})

	return errorsx.Wrap(err, "unable to run the workload's container")
}

func (t module) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, environsecret *cmdopts.EnvironSecret) (err error) {
	var (
		ws     workspaces.Context
		aid    = envx.String(uuidx.Nil().String(), eg.EnvComputeAccountID)
		uid    = envx.String(uuidx.Nil().String(), eg.EnvComputeRunID)
		descr  = envx.String("", eg.EnvComputeVCS)
		cc     grpc.ClientConnInterface
		cmdenv []string
	)

	// ensure when we run modules our umask is set to allow git clones to work properly
	runtimex.Umask(0002)

	if ws, err = workspaces.FromEnv(gctx.Context, t.Dir, t.Module); err != nil {
		return err
	}

	debugx.Println(spew.Sdump(ws))

	// workloads that were provided gcp credentials use them as the application default credentials of the unprivileged user.
	// this must happen before the environment of the module and its commands is built.
	errorsx.Log(errorsx.Wrap(installcredentials(gctx.Context), "unable to install gcp credentials"))

	cmdenvb := envx.Build().FromEnv(
		"PATH",
		"TERM",
		"COLORTERM",
		"LANG",
		"CI",
		eg.EnvComputeBin,
		eg.EnvComputeRunID,
		eg.EnvComputeAccountID,
		eg.EnvComputePodmanWorkload, // the commands run within the workload as well, see runners.PodmanWorkload.
	).Var(
		eg.EnvComputeWorkingDirectory, eg.DefaultWorkingDirectory(),
	).Var(
		eg.EnvComputeCacheDirectory, envx.String(eg.DefaultCacheDirectory(), eg.EnvComputeCacheDirectory, userx.EnvCacheDirectory),
	).Var(
		eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory(),
	).Var(
		eg.EnvComputeWorkspaceDirectory, eg.DefaultWorkspaceDirectory(),
	).Var(
		"PAGER", "cat", // no paging in this environmenet.
	)

	// authenticates the module's commands to the environ service of this module's socket, see interp/environ.
	environtoken, err := environ.NewToken(environsecret.Source(), uid, envx.Duration(time.Hour, eg.EnvComputeTTL))
	if err != nil {
		return errorsx.Wrap(err, "unable to generate environ token")
	}

	// the commands and modules of this module share its terminal, each is given one of its own. see ttyx.Mux.
	mux := ttyx.NewMux(gctx.Context)
	defer func() { errorsx.Log(errorsx.Wrap(mux.Close(), "unable to restore the terminal")) }()

	// must happen before the module starts anything, i.e.) bindfs, so the root cgroup is left empty and
	// the workload's containers get the cgroup controllers they would on the host.
	// not fatal: without it only containers within the workload lose cgroup support.
	// every module container is a new cgroup namespace regardless of its nested level, it does nothing
	// when the process was already moved out of the root cgroup, i.e.) by a module of a lower level.
	log.Println("isolating cgroups initiated")
	errorsx.Log(errorsx.Wrap(runners.IsolateCgroups(uid), "unable to isolate cgroups"))
	log.Println("isolating cgroups completed")

	if mlevel := envx.Int(0, eg.EnvComputeModuleNestedLevel); mlevel == 0 {
		var (
			control   net.Listener
			db        *sql.DB
			vmemlimit int64
		)

		// automatically detect the correct number of max procs for the module
		if _, err = maxprocs.Set(maxprocs.Logger(log.Printf)); err != nil {
			return errorsx.Wrap(err, "unable to set cpu limits")
		}

		if vmemlimit, err = memlimit.Set(memlimitx.Chain(memlimit.FromCgroup, memlimit.FromSystem, memlimit.Limit(bytesx.GiB)), memlimit.WithLogger(slog.Default())); err != nil {
			return errorsx.Wrap(err, "unable to set max limits")
		}

		// advertise the detected resources to the module's commands, and nested modules.
		cmdenvb = cmdenvb.Var(eg.EnvComputeMemoryLimit, strconv.FormatInt(vmemlimit, 10))
		cmdenvb = cmdenvb.Var(eg.EnvComputeCoresLimit, strconv.Itoa(runtime.GOMAXPROCS(-1)))

		// ensure the environment is up and running.
		if err = systemReady(gctx.Context); err != nil {
			return errorsx.Wrap(err, "failed system ready check")
		}

		debugx.Println("---------------------------- ROOT MODULE INITIATED ----------------------------")
		debugx.Println("module pid", os.Getpid())
		debugx.Println("account", aid)
		debugx.Println("run id", uid)
		debugx.Println("repository", descr)
		debugx.Println("number of cores (GOMAXPROCS - inaccurate)", runtime.GOMAXPROCS(-1))
		debugx.Println("ram available", bytesx.Unit(vmemlimit))
		debugx.Println("logging level", gctx.Verbosity)
		// fsx.PrintDir(os.DirFS(t.RuntimeDir))
		defer debugx.Println("---------------------------- ROOT MODULE COMPLETED ----------------------------")

		cspath := filepath.Join(t.RuntimeDir, eg.SocketControl)
		if control, err = net.Listen("unix", cspath); err != nil {
			return errorsx.Wrapf(err, "unable to create %s", cspath)
		}
		defer control.Close()

		if db, err = sql.Open("duckdb", filepath.Join(t.RuntimeDir, runners.DaemonAnalyticsName)); err != nil {
			return errorsx.Wrap(err, "unable to create analytics.db")
		}
		defer db.Close()

		if err = events.PrepareDB(gctx.Context, db); err != nil {
			return errorsx.Wrap(err, "unable to prepare analytics.db")
		}

		recordDiscoveredWorkloads(gctx.Context, db, eg.DefaultModuleDirectory(t.Dir), os.Environ()...)
		dpsrv := duckserve.New(db, duckserve.WithAcquireTimeout(time.Minute))
		go func() {
			dpspath := filepath.Join(t.RuntimeDir, eg.SocketAnalytics)
			if err := duckserve.ListenUnix(gctx.Context, dpspath, dpsrv); err != nil && gctx.Context.Err() == nil {
				log.Println("duckproxy:", err)
			}
		}()

		// postgres wire frontend for clients that aren't written in go. it is a client of the native server above.
		pgdb, err := sql.Open("duckproxy", filepath.Join(t.RuntimeDir, eg.SocketAnalytics))
		if err != nil {
			return errorsx.Wrap(err, "unable to create analytics pgwire")
		}
		defer pgdb.Close()

		pgsrv := duckgres.New(pgdb, duckgres.WithAcquireTimeout(time.Minute))
		go func() {
			pgpath := filepath.Join(t.RuntimeDir, eg.SocketAnalyticsPG)
			if err := duckgres.ListenUnix(gctx.Context, pgpath, pgsrv); err != nil && gctx.Context.Err() == nil {
				log.Println("duckpgwire:", err)
			}
		}()

		cmdenvb = cmdenvb.Var(
			eg.EnvComputeModuleSocket, eg.DefaultMountRoot(eg.RuntimeDirectory, filepath.Base(cspath)),
		).Var(
			eg.EnvComputeEnvironToken, environtoken,
		).FromEnviron(
			os.Environ()...,
		)
		if cmdenv, err = cmdenvb.Environ(); err != nil {
			return errorsx.Wrap(err, "unable to generate command environment")
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
		dbservice.NewService(gctx.Context, t.RuntimeDir).Bind(srv)
		execproxy.Auto(gctx.Context, t.Dir, t.RuntimeDir, cmdenv).Bind(srv)
		environ.NewService(filepath.Join(t.RuntimeDir, eg.EnvironFile), environsecret.Source()).Bind(srv)

		gpu, err := runners.AgentOptionGPU(envx.Boolean(false, eg.EnvComputeGPU))
		if err != nil {
			return errorsx.Wrap(err, "unable to configure gpu support")
		}

		ragent := runners.NewRunner(
			gctx.Context,
			ws,
			uid,
			runners.AgentOptionCommandLine("--env-file", eg.DefaultRuntimeDirectory(eg.EnvironFile)), // required for tty to work correct in local mode.
			runners.AgentOptionEnv(eg.EnvComputeTLSInsecure, strconv.FormatBool(tlsc.Insecure)),
			// nested modules share the repository cache, it is under the mount root mounted below. blank disables it.
			runners.AgentOptionEnv(eg.EnvComputeRepoCache, envx.String("", eg.EnvComputeRepoCache)),
			runners.AgentOptionVolumes(
				runners.AgentMountReadWrite("/root", "/root"),
				runners.AgentMountReadWrite(eg.DefaultMountRoot(), eg.DefaultMountRoot()),
				runners.AgentMountReadWrite("/var/lib/containers", "/var/lib/containers"),
			),
			// a machine of another OS than its host's has no egbin, and its own eg runs its containers.
			runners.AgentOptionEGBin(fsx.LocateFirst(eg.DefaultMountRoot(eg.RuntimeDirectory, eg.BinaryBin), errorsx.Must(exec.LookPath(os.Args[0])))),
			runners.AgentOptionHostOS(),
			runners.AgentOptionCommandLine("--cgroups", envx.String("enabled", eg.EnvComputeRunnerCgroups)),
			gpu,
		)

		c8sproxy.NewServiceProxy(
			log.Default(),
			ws,
			c8sproxy.ServiceProxyOptionCommandEnviron(cmdenv...),
			c8sproxy.ServiceProxyOptionContainerOptions(
				ragent.Options()...,
			),
			// within a workload podman's configuration gives the module's containers namespaces of their own, its builds share the module's network, see runners.PodmanWorkload.
			c8sproxy.ServiceProxyOptionBuildOptions(runners.PodmanWorkloadBuildOptions()...),
			c8sproxy.ServiceProxyOptionMux(mux),
		).Bind(srv)
		// a machine shares the container storage of the module booting it, as a container does, as overlay
		// can not layer it on the machine's root.
		machines.RegisterMachinesServer(srv, runners.NewMachines(
			tlsc.DefaultClient(),
			ws,
			log.Writer(),
			runners.MachineOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0]))),
			runners.MachineOptionVM(egvm.OptionMountVolume("/var/lib/containers", "/var/lib/containers")),
		))

		go func() {
			errorsx.Log(errorsx.Wrap(srv.Serve(control), "unable to serve control socket"))
		}()

		// TODO: investigate further tonight. this token might be getting clobbered by the auth client.
		if err = gitx.AutomaticCredentialRefresh(gctx.Context, tlsc.DefaultClient(), t.RuntimeDir, envx.String("", gitx.EnvAuthEGAccessToken)); err != nil {
			return errorsx.Wrap(err, "failed to refresh git credentials")
		}
	} else {
		var (
			control net.Listener
		)

		mspath := filepath.Join(t.RuntimeDir, eg.SocketModule())
		if control, err = net.Listen("unix", mspath); err != nil {
			return errorsx.Wrapf(err, "unable to create %s", mspath)
		}
		defer control.Close()

		debugx.Printf("---------------------------- MODULE INITIATED %d ----------------------------\n", mlevel)
		debugx.Fn(func() { envx.Debug(os.Environ()...) })
		debugx.Println("module pid", os.Getpid())
		debugx.Println("account", aid)
		debugx.Println("run id", uid)
		debugx.Println("repository", descr)
		debugx.Println("number of cores", runtime.GOMAXPROCS(-1))
		debugx.Println("logging level", gctx.Verbosity)
		debugx.Println("module pid", os.Getpid())
		debugx.Println("mspath", mspath)
		defer debugx.Printf("---------------------------- MODULE COMPLETED %d ----------------------------\n", mlevel)

		srv := grpc.NewServer(
			grpc.Creds(insecure.NewCredentials()), // this is a local socket
		)
		defer srv.GracefulStop()

		cmdenvb = cmdenvb.Var(
			eg.EnvComputeModuleSocket, eg.DefaultMountRoot(eg.RuntimeDirectory, filepath.Base(mspath)),
		).Var(
			eg.EnvComputeEnvironToken, environtoken,
		).FromEnviron(
			os.Environ()...,
		)

		if cmdenv, err = cmdenvb.Environ(); err != nil {
			return errorsx.Wrap(err, "failed to generate module command environment")
		}

		execproxy.Auto(gctx.Context, t.Dir, t.RuntimeDir, cmdenv).Bind(srv)
		environ.NewService(filepath.Join(t.RuntimeDir, eg.EnvironFile), environsecret.Source()).Bind(srv)
		go func() {
			errorsx.Log(errorsx.Wrap(srv.Serve(control), "unable to serve control socket"))
		}()
	}

	// IMPORTANT: duckdb does not play well with bindfs mounting the folders before
	// creating extensions/tables, it would nuke the working directory. it *mostly* worked once we moved this mount
	// call after duckdb. we're leaving the call here for now but it shouldn't matter.
	// and we're keen to remove it.
	if err = t.mounthack(gctx.Context, uid, ws); err != nil {
		return errorsx.Wrap(err, "unable to mount with correct permissions - this is a transient error that happens occassional likely due to bindfs/podman bugs")
	}

	if err = t.waylandhack(gctx.Context); err != nil {
		return errorsx.Wrap(err, "unable to configure wayland passthrough")
	}

	if cc, err = daemons.AutoRunnerClient(gctx, ws, uid, runners.AgentOptionAutoEGBin()); err != nil {
		return errorsx.Wrap(err, "auto runner client failed")
	}

	if pmode := envx.String("", eg.EnvComputeProfileMode); stringsx.Present(pmode) {
		pfile := filepath.Join(ws.CacheDir, ".eg", ".profiles", aid, uid, strconv.Itoa(envx.Int(0, eg.EnvComputeModuleNestedLevel)), fmt.Sprintf("%s.pprof", pmode))
		log.Println("profile will be written to", pfile)
		go func() {
			errorsx.Log(gdx.RecordFile(gctx.Context, pfile, gdx.Profile(gctx.Context, gdx.ProfileModeFromString(pmode))))
			log.Println("profile written to", pfile)
		}()
	}

	// a workload uploaded with its own Containerfile, the runner sets isolate to it, runs isolated within this container,
	// the root container of the runner, so what it builds and stores is accounted for with the rest of the workload's storage.
	if t.Isolate != "" {
		return t.isolated(gctx.Context, cc, uid, t.Isolate)
	}

	return interp.Remote(
		gctx.Context,
		ws,
		aid,
		uid,
		cc,
		t.Module,
		interp.OptionEnviron(cmdenv...),
		interp.OptionHTTPClient(tlsc.DefaultClient()),
		interp.OptionRepoCache(envx.From[string](eg.EnvComputeRepoCache)),
		interp.OptionMux(mux),
		interp.OptionArgs(t.Args...),
	)
}
