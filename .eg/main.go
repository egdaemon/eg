package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

func Test(ctx context.Context, _ eg.Op) error {
	return shell.Run(
		ctx,
		shell.New("ls -lha /opt/eg"),
		shell.Newf(
			"chmod -R 0770 %s",
			egenv.RootDirectory(),
		).Privileged(),
		shell.New("ls -lha /opt/eg"),
		shell.New("git fsck --full | head -25").Privileged(),
		shell.New("git fsck --full | head -25"),
		shell.New("git status --porcelain"),
	)
}

func Prepare(ctx context.Context, _ eg.Op) error {
	return shell.Run(
		ctx,
		shell.Newf("git config safe.directory"),
		shell.Newf("git config --global core.sharedRepository group"),
	)
}

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ltime)
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		Prepare,
		eggit.AutoClone,
		eg.Build(
			eg.DefaultModule(),
		),
		Test,
		eggolang.AutoCompile(
			eggolang.CompileOption.Tags("no_duckdb_arrow"),
			// eggolang.CompileOption.Debug(true),
		),
		eg.Module(
			ctx,
			eg.DefaultModule(),
			Test,
			eggolang.AutoCompile(
				eggolang.CompileOption.Tags("no_duckdb_arrow"),
				eggolang.CompileOption.Debug(true),
			),
			// eggolang.AutoTest(
			// 	eggolang.TestOption.Tags("no_duckdb_arrow"),
			// ),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
