package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"sync"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/james-lawrence/eg/cmd/cmderrors"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/cmd/eg/accountcmds"
	"github.com/james-lawrence/eg/internal/contextx"
	"github.com/james-lawrence/eg/internal/debugx"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/internal/fsx"
	"github.com/james-lawrence/eg/internal/osx"
	"github.com/james-lawrence/eg/internal/stringsx"
	"github.com/james-lawrence/eg/internal/userx"
	"github.com/willabides/kongplete"
)

func main() {
	var shellcli struct {
		cmdopts.Global
		Version            cmdopts.Version              `cmd:"" help:"display versioning information"`
		Monitor            monitor                      `cmd:"" help:"execute the interpreter and monitor the progress"`
		Interp             runner                       `cmd:"" help:"execute the interpreter on the given directory"`
		Module             module                       `cmd:"" help:"executes a compiled module directly" hidden:"true"`
		Daemon             daemon                       `cmd:"" help:"run in daemon mode letting the control plane push jobs to the local machine" hidden:"true"`
		AgentManagement    actlcmd                      `cmd:"" name:"actl" help:"agent management commands"`
		Register           accountcmds.Register         `cmd:"" name:"register" help:"register with an account with eg"`
		Login              accountcmds.Login            `cmd:"" name:"login" help:"login to a profile"`
		InstallCompletions kongplete.InstallCompletions `cmd:"" help:"install shell completions"`
	}

	var (
		err          error
		ctx          *kong.Context
		autorootuser = user.User{
			Gid:     "0",
			Uid:     "0",
			HomeDir: "/root",
		}
	)

	shellcli.Cleanup = &sync.WaitGroup{}
	shellcli.Context = contextx.WithWaitGroup(context.Background(), shellcli.Cleanup)
	shellcli.Context, shellcli.Shutdown = context.WithCancel(shellcli.Context)

	log.SetFlags(log.Flags() | log.Lshortfile)
	go debugx.DumpOnSignal(shellcli.Context, syscall.SIGUSR2)
	go cmdopts.Cleanup(shellcli.Context, shellcli.Shutdown, shellcli.Cleanup, func() {
		log.Println("waiting for systems to shutdown")
	}, os.Kill, os.Interrupt)

	user := userx.CurrentUserOrDefault(autorootuser)

	parser := kong.Must(
		&shellcli,
		kong.Name("eg"),
		kong.Description("cli for eg"),
		kong.Vars{
			"vars_cwd":             osx.Getwd("."),
			"vars_cache_directory": envx.String(os.TempDir(), "CACHE_DIRECTORY", "XDG_CACHE_HOME"),
			"vars_account_id":      envx.String("", "EG_ACCOUNT"),
			"vars_ssh_key_path":    fsx.LocateFirstInDir(filepath.Join(user.HomeDir, ".ssh"), "id_ed25519", "id"),
			"vars_user_name":       stringsx.DefaultIfBlank(user.Name, user.Username),
			"vars_user_username":   user.Username,
		},
		kong.UsageOnError(),
		kong.Bind(&shellcli.Global),
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

	if err = ctx.Run(); err != nil {
		log.Println(cmderrors.Sprint(err))
	}

	shellcli.Shutdown()
	shellcli.Cleanup.Wait()
}
