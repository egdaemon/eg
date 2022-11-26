package main

import (
	"context"
	"log"
	"net"
	"os"
	"reflect"
	"sync"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/james-lawrence/eg/cmd/cmderrors"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/internal/contextx"
	"github.com/james-lawrence/eg/internal/debugx"
	"github.com/james-lawrence/eg/internal/osx"
	"github.com/willabides/kongplete"
)

func main() {
	var shellcli struct {
		cmdopts.Global
		Version            cmdopts.Version              `cmd:"" help:"display versioning information"`
		Interp             runner                       `cmd:"" help:"execute the interpreter on the given directory"`
		InstallCompletions kongplete.InstallCompletions `cmd:"" help:"install shell completions"`
	}

	var (
		err error
		ctx *kong.Context
	)

	shellcli.Cleanup = &sync.WaitGroup{}
	shellcli.Context = contextx.WithWaitGroup(context.Background(), shellcli.Cleanup)
	shellcli.Context, shellcli.Shutdown = context.WithCancel(shellcli.Context)

	log.SetFlags(log.Flags() | log.Lshortfile)
	go debugx.DumpOnSignal(shellcli.Context, syscall.SIGUSR2)
	go cmdopts.Cleanup(shellcli.Context, shellcli.Shutdown, shellcli.Cleanup, func() {
		log.Println("waiting for systems to shutdown")
	}, os.Kill, os.Interrupt)

	parser := kong.Must(
		&shellcli,
		kong.Name("eg"),
		kong.Description("cli for eg"),
		kong.Vars{
			"vars_cwd": osx.Getwd("."),
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
		shellcli.Shutdown()
	}

	shellcli.Cleanup.Wait()

	if err != nil {
		os.Exit(1)
	}
}
