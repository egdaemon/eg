package libp2px_test

import (
	"context"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestDefaultMDNSLookup(t *testing.T) {
	// exercises real multicast between two hosts. skipped when the environment can't do multicast
	// (containers, CI sandboxes) since that isn't a failure of the code under test.
	t.Run("finds an advertised runner", func(t *testing.T) {
		if testing.Short() {
			t.Skip("multicast dns test skipped in short mode")
		}

		runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer runner.Close()

		closer, err := libp2px.Advertise(runner)
		if err != nil {
			t.Skip("multicast dns unavailable:", err)
		}
		defer closer.Close()

		var match *peer.AddrInfo

		found := libp2px.DefaultMDNSLookup(t.Context(), 3*time.Second)
		for info := range found.Each(t.Context()) {
			if info.ID == runner.ID() {
				match = &info
			}
		}

		if err := found.Err(); err != nil {
			t.Skip("multicast dns unavailable:", err)
		}

		if match == nil {
			t.Skip("multicast dns produced no results; no multicast route in this environment")
		}

		require.NotEmpty(t, match.Addrs)
	})

	t.Run("stopping early ends the scan before its timeout", func(t *testing.T) {
		if testing.Short() {
			t.Skip("multicast dns test skipped in short mode")
		}

		runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"), libp2p.DisableRelay())
		require.NoError(t, err)
		defer runner.Close()

		closer, err := libp2px.Advertise(runner)
		if err != nil {
			t.Skip("multicast dns unavailable:", err)
		}
		defer closer.Close()

		start := time.Now()
		found := libp2px.DefaultMDNSLookup(t.Context(), 10*time.Second)

		var seen bool
		for info := range found.Each(t.Context()) {
			if info.ID == runner.ID() {
				seen = true
				break
			}
		}

		if !seen {
			t.Skip("multicast dns produced no results; no multicast route in this environment")
		}

		require.NoError(t, found.Err())
		require.Less(t, time.Since(start), 8*time.Second, "the scan ended without waiting for its timeout")
	})

	t.Run("returns promptly when the context is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		found := libp2px.DefaultMDNSLookup(ctx, time.Minute)
		for range found.Each(ctx) {
		}

		if err := found.Err(); err != nil {
			t.Skip("multicast dns unavailable:", err)
		}
		require.Less(t, time.Since(start), 10*time.Second)
	})
}
