package profilex

import (
	"context"

	"github.com/egdaemon/eg/internal/contextx"
)

type Stoppable interface {
	Stop()
}

func Run(ctx context.Context, p Stoppable) error {
	defer p.Stop()
	<-ctx.Done()
	return contextx.IgnoreDeadlineExceeded(ctx.Err())
}

type StopFunc func()

func (t StopFunc) Stop() {
	t()
}

func Noop() Stoppable {
	return StopFunc(func() {})
}
