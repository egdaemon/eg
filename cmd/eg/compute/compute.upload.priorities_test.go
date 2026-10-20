package compute

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPriorities(t *testing.T) {
	for _, c := range []struct {
		name   string
		lan    int
		direct int
		want   []strategy
	}{
		{name: "defaults try the local network first", lan: 2, direct: 1, want: []strategy{strategyLAN, strategyDirect}},
		{name: "higher goes first", lan: 1, direct: 5, want: []strategy{strategyDirect, strategyLAN}},
		{name: "ties favor the local network", lan: 3, direct: 3, want: []strategy{strategyLAN, strategyDirect}},
		{name: "zero disables lan", lan: 0, direct: 1, want: []strategy{strategyDirect}},
		{name: "zero disables direct", lan: 2, direct: 0, want: []strategy{strategyLAN}},
		{name: "negative disables", lan: -1, direct: -4, want: []strategy{}},
		{name: "both disabled is upload only", lan: 0, direct: 0, want: []strategy{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, priorities(c.lan, c.direct))
		})
	}
}
