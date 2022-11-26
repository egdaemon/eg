package systemx

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
)

// Cleanup - waits for one of the provided signals, or for the provided context's
// done event to be received. Once received the cleanup function is executed and
// blocks while it waits for everything to finish
func Cleanup(ctx context.Context, cancel func(), wg *sync.WaitGroup, sigs ...os.Signal) func(func()) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, sigs...)

	return func(cleanup func()) {
		select {
		case <-ctx.Done():
		case s := <-signals:
			log.Println("signal received", s.String())
			cancel()
		}

		signal.Stop(signals)
		close(signals)

		cleanup()
	}
}
