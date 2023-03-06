package main

import (
	"github.com/james-lawrence/eg/cmd/eg/actl"
)

type actlcmd struct {
	Authorize actl.AuthorizeAgent `cmd:"" help:"authorize agents"`
}
