package cmdopts

import (
	"context"
	"sync"
)

type Global struct {
	Verbosity int                `help:"increase verbosity of logging" short:"v" type:"counter" default:"0"`
	Context   context.Context    `kong:"-"`
	Shutdown  context.CancelFunc `kong:"-"`
	Cleanup   *sync.WaitGroup    `kong:"-"`
}
