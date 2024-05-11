package main

import (
	"github.com/egdaemon/eg/cmd/eg/actl"
)

type actlcmd struct {
	Authorize actl.AuthorizeAgent `cmd:"" help:"authorize agents"`
	Bootstrap actl.Bootstrap      `cmd:"" help:"helpful functions for bootstrapping the system"`
}
