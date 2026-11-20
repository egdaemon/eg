package compute

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/libp2px"
)

type cacheExec struct {
	cacheoptions `embed:""`
	Command      []string `arg:"" name:"command" passthrough:"" help:"command and arguments to execute within the cache root, i.e.) tree -L 3, flags for eg must come before the command"`
}

func (t cacheExec) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T, find libp2px.MDNSLookup) (err error) {
	var (
		encoded []byte
	)

	if len(t.Command) == 0 {
		return fmt.Errorf("a command is required, i.e.) eg compute cache exec ls -lha")
	}

	if encoded, err = json.Marshal(t.Command); err != nil {
		return errorsx.Wrap(err, "unable to encode the command")
	}

	return t.cacheoptions.run(
		gctx, tlsc, ident, find,
		"cache/exec",
		fmt.Sprintf("cache exec - %s", strings.Join(t.Command, " ")),
		[]string{envx.Format(cacheEnvExecArgs, string(encoded))},
	)
}
