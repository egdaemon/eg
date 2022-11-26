package main

import (
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/interp"
)

type runner struct {
	Dir string `name:"directory" help:"directory to load" default:"${vars_cwd}"`
}

func (t runner) Run(ctx *cmdopts.Global) (err error) {
	return interp.Run(ctx.Context, t.Dir)
}
