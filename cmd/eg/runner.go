package main

import (
	"path/filepath"

	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/compile"
	"github.com/james-lawrence/eg/interp"
)

type runner struct {
	Dir string `name:"directory" help:"directory to load" default:"${vars_cwd}"`
}

func (t runner) Run(ctx *cmdopts.Global) (err error) {
	if err = compile.Run(ctx.Context, "interp/runtime/wasi/main.go", filepath.Join(t.Dir, "eg0.runtime.wasm")); err != nil {
		return err
	}

	if err = compile.Run(ctx.Context, "examples/example1/main.go", filepath.Join(t.Dir, "eg1.example.wasm")); err != nil {
		return err
	}

	return interp.Run(ctx.Context, t.Dir)
}
