package lan_test

import (
	"testing"

	"github.com/egdaemon/eg/internal/lan"
	"github.com/stretchr/testify/require"
)

func TestPeerLabelList(t *testing.T) {
	require.Equal(t, []string{"a", "b"}, lan.Peer{Labels: `["a","b"]`}.LabelList())
	require.Empty(t, lan.Peer{Labels: "not json"}.LabelList())
}
