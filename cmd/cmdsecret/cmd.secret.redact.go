package cmdsecret

import (
	"fmt"
	"io"
	"os"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/uuidx"
)

type CmdRedact struct {
	Environment []string `name:"env" short:"e" help:"additional environment variables to redact"`
	Seed        string   `name:"seed" help:"seed for the digest of redacted values, the same seed and value always produce the same digest. defaults to a random uuid"`
}

func (t CmdRedact) Run(gctx *cmdopts.Global) error {
	return redact(os.Stdout, os.Stdin, stringsx.First(t.Seed, uuidx.NewV4().String()), t.Environment...)
}

// redact reads KEY=VALUE environment variables from src and writes them to dst with the values of eg's
// sensitive variables and the additional keys replaced by redacted:{digest}.
func redact(dst io.Writer, src io.Reader, seed string, keys ...string) error {
	if err := envx.Redact(envx.Build().FromReader(src), seed, append(eg.SensitiveEnvironment(), keys...)...).CopyTo(dst); err != nil {
		return fmt.Errorf("failed to redact environment: %w", err)
	}

	return nil
}
