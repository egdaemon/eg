package main

import (
	"context"
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/dustin/go-humanize"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmderrors"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdplete"
	"github.com/egdaemon/eg/cmd/eg/accountcmds"
	"github.com/egdaemon/eg/cmd/eg/compute"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/contextx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/osx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/tracex"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/go-git/go-git/v5"
	"github.com/gofrs/uuid"
	"github.com/willabides/kongplete"

	// ensure its a dependency.
	_ "github.com/egdaemon/wasinet/wasinet"
)

func machineID() string {
	var (
		err error
		raw []byte
	)

	midpath := filepath.Join(userx.DefaultCacheDirectory(), "machine-id")

	if err = os.MkdirAll(filepath.Dir(midpath), 0700); err != nil {
		panic(errorsx.Wrapf(err, "unable to ensure cache directory for machine id %s", midpath))
	}

	if raw, err = os.ReadFile(midpath); err == nil {
		return strings.TrimSpace(string(raw))
	}

	// log.Println("failed to read a valid machine id, generating a random uuid", err)
	uid := uuid.Must(uuid.NewV7()).String()
	if err = os.WriteFile(midpath, []byte(uid), 0600); err == nil {
		return strings.TrimSpace(uid)
	}

	panic(errorsx.Wrapf(err, "failed to generate a machine id at %s", midpath))
}

func main() {
	var shellcli struct {
		cmdopts.Hotswap
		cmdopts.Global
		cmdopts.TLSConfig
		Version            cmdopts.Version              `cmd:"" help:"display versioning information"`
		Compute            compute.Cmd                  `cmd:"" help:"commands for running compute workloads"`
		Module             module                       `cmd:"" help:"executes a compiled module directly" hidden:"true"`
		Wasi               wasiCmd                      `cmd:"" help:"run a standalone wasi module" hidden:"true"`
		Daemon             daemon                       `cmd:"" help:"run in daemon mode letting the control plane push jobs to machines" hidden:"true"`
		AgentManagement    actlcmd                      `cmd:"" name:"actl" help:"agent management commands"`
		Register           accountcmds.Signup           `cmd:"" name:"register" help:"register with an account with eg"`
		Login              accountcmds.Login            `cmd:"" name:"login" help:"login to a profile"`
		Browser            accountcmds.OTP              `cmd:"" name:"browser" help:"login to the browser console"`
		Ident              accountcmds.Identity         `cmd:"" name:"iden" help:"display current credentials"`
		DiskUsage          daemons.DiskUsage            `cmd:"" name:"disk-usage" help:"monitors disk usage and executes services when above threshold"`
		InstallCompletions kongplete.InstallCompletions `cmd:"" help:"install shell completions"`
	}

	var (
		err error
		ctx *kong.Context
	)

	shellcli.Cleanup = &sync.WaitGroup{}
	shellcli.Context = contextx.WithWaitGroup(context.Background(), shellcli.Cleanup)
	shellcli.Context, shellcli.Shutdown = context.WithCancelCause(shellcli.Context)
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	go debugx.DumpOnSignal(shellcli.Context, syscall.SIGUSR2)
	go cmdopts.Cleanup(shellcli.Context, shellcli.Shutdown, shellcli.Cleanup, func() {
		log.Println("waiting for systems to shutdown")
	}, os.Kill, os.Interrupt)

	user := userx.CurrentUserOrDefault(userx.Root())
	gitdir := envx.String(gitx.DetectRoot(), "EG_GIT_REPOSITORY")

	parser := kong.Must(
		&shellcli,
		kong.Name("eg"),
		kong.Description("cli for eg"),
		kong.Vars{
			"vars_timestamp_started":    time.Now().UTC().Format(time.RFC3339),
			"vars_endpoint":             eg.EnvAPIHostDefault(),
			"vars_console_endpoint":     eg.EnvConsoleHostDefault(),
			"vars_tls_insecure_default": eg.EnvTLSInsecure(),
			"vars_cwd":                  osx.Getwd("."),
			"vars_git_directory":        gitdir,
			"vars_cache_directory":      userx.DefaultCacheDirectory(),
			"vars_runtime_directory":    userx.DefaultRuntimeDirectory(),
			"vars_eg_runtime_directory": eg.DefaultMountRoot(eg.RuntimeDirectory),
			"vars_account_id":           envx.String("", "EG_ACCOUNT"),
			"vars_machine_id":           envx.String(machineID(), "EG_MACHINE_ID"),
			"vars_entropy_seed":         envx.String(errorsx.Must(uuid.NewV4()).String(), "EG_ENTROPY_SEED"),
			"vars_ssh_key_path": filepath.Join(
				envx.String(
					filepath.Join(userx.HomeDirectoryOrDefault(user.HomeDir), ".ssh"),
					"CONFIGURATION_DIRECTORY",
				),
				userx.DefaultDir,
			),
			"vars_ssh_known_hosts_path":    filepath.Join(userx.HomeDirectoryOrDefault(user.HomeDir), ".ssh", "known_hosts"),
			"vars_user_name":               stringsx.DefaultIfBlank(user.Name, user.Username),
			"vars_user_username":           user.Username,
			"vars_user_home":               userx.HomeDirectoryOrDefault(user.HomeDir),
			"vars_os":                      runtime.GOOS,
			"vars_arch":                    runtime.GOARCH,
			"vars_cores_minimum_default":   strconv.FormatUint(envx.Uint64(uint64(float64(runtime.NumCPU())*0.8), "EG_RESOURCES_CORES"), 10),
			"vars_memory_minimum_default":  humanize.IBytes(envx.Uint64(0, "EG_RESOURCES_MEMORY")),
			"vars_disk_minimum_default":    humanize.IBytes(envx.Uint64(0, "EG_RESOURCES_DISK")),
			"vars_vram_minimum_default":    humanize.IBytes(envx.Uint64(0*bytesx.GiB, "EG_RESOURCES_VRAM")),
			"vars_git_default_remote_name": git.DefaultRemoteName,
			"vars_git_default_reference":   "main",
			"vars_git_head_reference":      "HEAD",
			"vars_workload_directory":      eg.DefaultModuleDirectory(),
		},
		kong.UsageOnError(),
		kong.Bind(
			&shellcli.Global,
			&shellcli.TLSConfig,
			new(cmdopts.HotswapPath),
		),
		kong.TypeMapper(reflect.TypeOf(&net.IP{}), kong.MapperFunc(cmdopts.ParseIP)),
		kong.TypeMapper(reflect.TypeOf(&net.TCPAddr{}), kong.MapperFunc(cmdopts.ParseTCPAddr)),
		kong.TypeMapper(reflect.TypeOf([]*net.TCPAddr(nil)), kong.MapperFunc(cmdopts.ParseTCPAddrArray)),
	)

	pw := cmdplete.NewWorkload(filepath.Join(gitdir, eg.DefaultModuleDirectory()))

	// Run kongplete.Complete to handle completion requests
	kongplete.Complete(
		parser,
		kongplete.WithPredictor(
			"eg.workload",
			pw,
		),
	)

	if ctx, err = parser.Parse(os.Args[1:]); err != nil {
		log.Println(cmderrors.Sprint(err))
		os.Exit(1)
	}

	if envx.Boolean(false, eg.EnvLogsDebug) {
		log.Println("api host", eg.EnvAPIHostDefault())
		log.Println("console host", eg.EnvConsoleHostDefault())
		log.Println("container api host", eg.EnvContainerAPIHostDefault())
		log.Println("tls insecure", eg.EnvTLSInsecure())
	}

	if err = ctx.Run(); err != nil {
		log.Println(cmderrors.Sprint(err))
		os.Exit(1)
	}

	tracex.Println("shutting down")
	shellcli.Shutdown(nil)
	shellcli.Cleanup.Wait()
}
