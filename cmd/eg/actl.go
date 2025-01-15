package main

import (
	"github.com/egdaemon/eg/cmd/eg/actl"
)

type actlcmd struct {
	Authorize actl.AuthorizeAgent `cmd:"" help:"authorize agents"`
	Bootstrap actl.Bootstrap      `cmd:"" help:"functions for bootstrapping the eg both locally and on workload runners"`
}
