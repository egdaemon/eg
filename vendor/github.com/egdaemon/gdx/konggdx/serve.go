package konggdx

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"

	"github.com/egdaemon/gdx"
)

// Serve stands up the gdx debug HTTP surface on a unix socket.
type Serve struct {
	Socket string `flag:"" name:"socket" help:"unix socket path to bind" default:"${vars_gdx_socket}"`
}

func (t Serve) Run(ctx context.Context) error {
	os.Remove(t.Socket)

	l, err := net.Listen("unix", t.Socket)
	if err != nil {
		return fmt.Errorf("unable to bind gdx debug socket: %w", err)
	}

	return t.run(ctx, l)
}

func (t Serve) run(ctx context.Context, l net.Listener) error {
	defer l.Close()

	go func() {
		<-ctx.Done()
		l.Close()
	}()

	if err := http.Serve(l, gdx.NewHTTPFn(gdx.Options().FromEnv()...)); err != nil && ctx.Err() == nil {
		return err
	}

	return nil
}
