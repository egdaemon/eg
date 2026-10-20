package libp2px_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/libp2p/go-libp2p"
	"github.com/stretchr/testify/require"
)

func TestAdvertise(t *testing.T) {
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/0"), libp2p.DisableRelay())
	require.NoError(t, err)
	defer h.Close()

	closer, err := libp2px.Advertise(h)
	if err != nil {
		t.Skip("multicast dns unavailable:", err)
	}
	require.NoError(t, closer.Close())
}
