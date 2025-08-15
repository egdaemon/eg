package compute

import "embed"

type cmdbuiltin struct {
	Upload builtinUpload `cmd:"" help:"upload and run a builtin workload"`
	Local  builtinLocal  `cmd:"" help:"upload and run a builtin workload"`
}

//go:embed .builtin
var embeddedbuiltin embed.FS
