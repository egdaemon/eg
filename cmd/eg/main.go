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

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmderrors"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/accountcmds"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/contextx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/osx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/go-git/go-git/v5"
	"github.com/gofrs/uuid"
	"github.com/willabides/kongplete"
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
		cmdopts.Global
		cmdopts.TLSConfig
		cmdopts.RuntimeResources
		Version            cmdopts.Version              `cmd:"" help:"display versioning information"`
		Monitor            monitor                      `cmd:"" help:"execute the interpreter and monitor the progress" hidden:"true"`
		Local              runner                       `cmd:"" help:"execute the interpreter on the given directory" hidden:"true"`
		Upload             upload                       `cmd:"" help:"compiles and uploads the process to the cluster"`
		Module             module                       `cmd:"" help:"executes a compiled module directly" hidden:"true"`
		Daemon             daemon                       `cmd:"" help:"run in daemon mode letting the control plane push jobs to machines" hidden:"true"`
		AgentManagement    actlcmd                      `cmd:"" name:"actl" help:"agent management commands"`
		Register           accountcmds.Register         `cmd:"" name:"register" help:"register with an account with eg"`
		Login              accountcmds.Login            `cmd:"" name:"login" help:"login to a profile"`
		Browser            accountcmds.OTP              `cmd:"" name:"browser" help:"login to the browser console"`
		InstallCompletions kongplete.InstallCompletions `cmd:"" help:"install shell completions"`
	}

	var (
		err error
		ctx *kong.Context
	)

	shellcli.Cleanup = &sync.WaitGroup{}
	shellcli.Context = contextx.WithWaitGroup(context.Background(), shellcli.Cleanup)
	shellcli.Context, shellcli.Shutdown = context.WithCancel(shellcli.Context)

	go debugx.DumpOnSignal(shellcli.Context, syscall.SIGUSR2)
	go cmdopts.Cleanup(shellcli.Context, shellcli.Shutdown, shellcli.Cleanup, func() {
		log.Println("waiting for systems to shutdown")
	}, os.Kill, os.Interrupt)

	user := userx.CurrentUserOrDefault(userx.Root())

	parser := kong.Must(
		&shellcli,
		kong.Name("eg"),
		kong.Description("cli for eg"),
		kong.Vars{
			"vars_endpoint":          envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost),
			"vars_cwd":               osx.Getwd("."),
			"vars_runtime_directory": userx.DefaultRuntimeDirectory(),
			"vars_cache_directory":   userx.DefaultCacheDirectory(),
			"vars_account_id":        envx.String("", "EG_ACCOUNT"),
			"vars_machine_id":        envx.String(machineID(), "EG_MACHINE_ID"),
			"vars_entropy_seed":      envx.String(errorsx.Must(uuid.NewV4()).String(), "EG_ENTROPY_SEED"),
			"vars_ssh_key_path": filepath.Join(
				envx.String(
					filepath.Join(userx.HomeDirectoryOrDefault(user.HomeDir), ".ssh"),
					"CACHE_DIRECTORY",
				),
				"eg",
			),
			"vars_user_name":               stringsx.DefaultIfBlank(user.Name, user.Username),
			"vars_user_username":           user.Username,
			"vars_os":                      runtime.GOOS,
			"vars_arch":                    runtime.GOARCH,
			"vars_cores_minimum_default":   strconv.FormatUint(envx.Uint64(1, "EG_RESOURCES_CORES"), 10),
			"vars_memory_minimum_default":  strconv.FormatUint(envx.Uint64(256*bytesx.MiB, "EG_RESOURCES_MEMORY"), 10),
			"vars_disk_minimum_default":    strconv.FormatUint(envx.Uint64(2*bytesx.GiB, "EG_RESOURCES_DISK"), 10),
			"vars_git_default_remote_name": git.DefaultRemoteName,
			"vars_git_default_reference":   "main",
		},
		kong.UsageOnError(),
		kong.Bind(
			&shellcli.Global,
			&shellcli.TLSConfig,
			&shellcli.RuntimeResources,
		),
		kong.TypeMapper(reflect.TypeOf(&net.IP{}), kong.MapperFunc(cmdopts.ParseIP)),
		kong.TypeMapper(reflect.TypeOf(&net.TCPAddr{}), kong.MapperFunc(cmdopts.ParseTCPAddr)),
		kong.TypeMapper(reflect.TypeOf([]*net.TCPAddr(nil)), kong.MapperFunc(cmdopts.ParseTCPAddrArray)),
	)

	// Run kongplete.Complete to handle completion requests
	kongplete.Complete(
		parser,
	)

	if ctx, err = parser.Parse(os.Args[1:]); err != nil {
		log.Println(cmderrors.Sprint(err))
		os.Exit(1)
	}

	if envx.Boolean(false, eg.EnvLogsDebug) {
		envx.Debug(os.Environ()...)
	}

	if err = ctx.Run(); err != nil {
		log.Println(cmderrors.Sprint(err))
	}

	shellcli.Shutdown()
	shellcli.Cleanup.Wait()
}
