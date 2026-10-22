package cmdopts

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
)

// Cleanup - waits for one of the provided signals, or for the provided context's
// done event to be received. Once received the cleanup function is executed and
// blocks while it waits for everything to finish
func Cleanup(ctx context.Context, cancel func(error), wg *sync.WaitGroup, cleanup func(), sigs ...os.Signal) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, sigs...)

	select {
	case <-ctx.Done():
		signal.Stop(signals)
	case s := <-signals:
		log.Println("signal received", s.String())
		cancel(fmt.Errorf("signal received: %s", s.String()))

		// keep handling signals while cleanup runs, a second one forces the exit
		// instead of falling back to the default behavior which skips deferred cleanup silently.
		go func() {
			s := <-signals
			log.Println("second signal received, forcing exit", s.String())
			os.Exit(1)
		}()
	}

	cleanup()
}
