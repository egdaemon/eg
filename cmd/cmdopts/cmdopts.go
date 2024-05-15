package cmdopts

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc/grpclog"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/grpcx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/tracex"
	"github.com/egdaemon/eg/runtime/wasi/env"
)

type Global struct {
	Verbosity int                `help:"increase verbosity of logging" short:"v" type:"counter" default:"0"`
	Context   context.Context    `kong:"-"`
	Shutdown  context.CancelFunc `kong:"-"`
	Cleanup   *sync.WaitGroup    `kong:"-"`
}

func (t Global) AfterApply() error {
	log.SetFlags(log.Flags() | log.Lshortfile)
	switch envx.Int(t.Verbosity, EnvLoggingVerbosity) {
	case 4: // NETWORK
		os.Setenv(eg.EnvLogsNetwork, "1")
	case 3: // TRACE
		tracex.SetOutput(os.Stderr)
		tracex.SetFlags(log.Flags())
		os.Setenv(eg.EnvLogsTrace, "1")
		fallthrough
	case 2: // DEBUG
		debugx.SetOutput(os.Stderr)
		debugx.SetFlags(log.Flags())
		os.Setenv(eg.EnvLogsDebug, "1")
		fallthrough
	case 1: // INFO
	default: // ERROR - minimal
	}

	// enable GRPC logging
	if envx.Boolean(false, eg.EnvLogsNetwork) {
		os.Setenv("GRPC_GO_LOG_VERBOSITY_LEVEL", "99")
		os.Setenv("GRPC_GO_LOG_SEVERITY_LEVEL", "info")
		grpclog.SetLoggerV2(grpcx.NewLogger())
	}

	return nil
}

type TLSConfig struct {
	Insecure bool `help:"allow unsigned (and therefor insecure) tls certificates to be used" name:"insecure" default:"false"`
}

func (t TLSConfig) Config() *tls.Config {
	return (&tls.Config{
		InsecureSkipVerify: t.Insecure,
	}).Clone()
}

func (t TLSConfig) DefaultClient() *http.Client {
	ctransport := &http.Transport{
		TLSClientConfig: t.Config(),
	}

	defaultclient := &http.Client{Transport: ctransport, Timeout: 20 * time.Second}

	if env.Boolean(false, eg.EnvLogsNetwork) {
		return httpx.DebugClient(defaultclient)
	}

	return defaultclient
}
