package cmdopts_test

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
)

type cleanupSignalsOptOut struct{}

func (cleanupSignalsOptOut) AfterApply(s *cmdopts.CleanupSignals) error {
	s.Remove(os.Interrupt)
	return nil
}

func (cleanupSignalsOptOut) Run() error { return nil }

type cleanupSignalsDefault struct{}

func (cleanupSignalsDefault) Run() error { return nil }

func TestCleanup(t *testing.T) {
	t.Run("remove drops the signal", func(t *testing.T) {
		s := &cmdopts.CleanupSignals{Sigs: []os.Signal{os.Kill, syscall.SIGTERM, os.Interrupt}}
		s.Remove(os.Interrupt)
		require.Equal(t, []os.Signal{os.Kill, syscall.SIGTERM}, s.Sigs)

		s.Remove(os.Interrupt)
		require.Equal(t, []os.Signal{os.Kill, syscall.SIGTERM}, s.Sigs, "removing a missing signal is a noop")
	})

	var cli struct {
		OptOut  cleanupSignalsOptOut  `cmd:""`
		Default cleanupSignalsDefault `cmd:""`
	}

	t.Run("kong hook removes the signal", func(t *testing.T) {
		s := &cmdopts.CleanupSignals{Sigs: []os.Signal{syscall.SIGTERM, os.Interrupt}}
		parser, err := kong.New(&cli, kong.Bind(s))
		require.NoError(t, err)
		_, err = parser.Parse([]string{"opt-out"})
		require.NoError(t, err)
		require.Equal(t, []os.Signal{syscall.SIGTERM}, s.Sigs)
	})

	t.Run("kong command without a hook keeps the defaults", func(t *testing.T) {
		s := &cmdopts.CleanupSignals{Sigs: []os.Signal{syscall.SIGTERM, os.Interrupt}}
		parser, err := kong.New(&cli, kong.Bind(s))
		require.NoError(t, err)
		_, err = parser.Parse([]string{"default"})
		require.NoError(t, err)
		require.Equal(t, []os.Signal{syscall.SIGTERM, os.Interrupt}, s.Sigs)
	})

	t.Run("registered signal cancels and runs cleanup", func(t *testing.T) {
		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)

		cleaned := make(chan struct{})
		go cmdopts.Cleanup(ctx, cancel, &sync.WaitGroup{}, func() { close(cleaned) }, syscall.SIGUSR1)

		// a second signal forces os.Exit, so send exactly one after Cleanup has registered.
		time.Sleep(50 * time.Millisecond)
		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGUSR1))
		select {
		case <-cleaned:
		case <-time.After(5 * time.Second):
			t.Fatal("cleanup did not run after the signal")
		}

		require.ErrorContains(t, context.Cause(ctx), "signal received")
	})

	t.Run("unregistered signal is ignored by cleanup", func(t *testing.T) {
		// catch SIGUSR2 so delivering it doesn't terminate the test process.
		catch := make(chan os.Signal, 1)
		signal.Notify(catch, syscall.SIGUSR2)
		defer signal.Stop(catch)

		ctx, cancel := context.WithCancelCause(context.Background())
		defer cancel(nil)

		cleaned := make(chan struct{})
		go cmdopts.Cleanup(ctx, cancel, &sync.WaitGroup{}, func() { close(cleaned) }, syscall.SIGUSR1)

		time.Sleep(50 * time.Millisecond) // let Cleanup register.
		require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGUSR2))
		select {
		case <-catch:
		case <-time.After(5 * time.Second):
			t.Fatal("signal was not delivered")
		}

		require.NoError(t, context.Cause(ctx), "cleanup must not cancel for a signal it did not register")
		select {
		case <-cleaned:
			t.Fatal("cleanup ran for an unregistered signal")
		default:
		}

		cancel(nil)
		select {
		case <-cleaned:
		case <-time.After(5 * time.Second):
			t.Fatal("cleanup did not run after context cancellation")
		}
	})
}
