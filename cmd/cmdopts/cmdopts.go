package cmdopts

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync"
	"time"
)

type Global struct {
	Verbosity int                `help:"increase verbosity of logging" short:"v" type:"counter" default:"0"`
	Context   context.Context    `kong:"-"`
	Shutdown  context.CancelFunc `kong:"-"`
	Cleanup   *sync.WaitGroup    `kong:"-"`
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
	return &http.Client{Transport: ctransport, Timeout: 10 * time.Second}
}
