package netx

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const routeHeader = "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n"

func TestParseDefaultRouteInterface(t *testing.T) {
	for _, c := range []struct {
		name     string
		table    string
		expected string
		err      bool
	}{
		{
			name: "single default route",
			table: routeHeader +
				"enp13s0\t00000000\t0128A8C0\t0003\t0\t0\t1024\t00000000\t0\t0\t0\n" +
				"enp13s0\t0028A8C0\t00000000\t0001\t0\t0\t1024\t00FFFFFF\t0\t0\t0\n",
			expected: "enp13s0",
		},
		{
			name: "lowest metric wins",
			table: routeHeader +
				"wlan0\t00000000\t0128A8C0\t0003\t0\t0\t600\t00000000\t0\t0\t0\n" +
				"enp13s0\t00000000\t0128A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n",
			expected: "enp13s0",
		},
		{
			name: "routes that are down are ignored",
			table: routeHeader +
				"enp13s0\t00000000\t0128A8C0\t0002\t0\t0\t100\t00000000\t0\t0\t0\n" +
				"wlan0\t00000000\t0128A8C0\t0003\t0\t0\t600\t00000000\t0\t0\t0\n",
			expected: "wlan0",
		},
		{
			name: "a route to a network is not the default",
			table: routeHeader +
				"podman1\t0028A8C0\t00000000\t0001\t0\t0\t0\t00FFFFFF\t0\t0\t0\n",
			err: true,
		},
		{
			name:  "no routes",
			table: routeHeader,
			err:   true,
		},
		{
			name:  "empty",
			table: "",
			err:   true,
		},
		{
			name: "malformed rows are skipped",
			table: routeHeader +
				"garbage\n" +
				"enp13s0\t00000000\t0128A8C0\tzz\t0\t0\t100\t00000000\t0\t0\t0\n" +
				"wlan0\t00000000\t0128A8C0\t0003\t0\t0\tnope\t00000000\t0\t0\t0\n" +
				"eth1\t00000000\t0128A8C0\t0003\t0\t0\t200\t00000000\t0\t0\t0\n",
			expected: "eth1",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			iface, err := parseDefaultRouteInterface(strings.NewReader(c.table))
			if c.err {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, c.expected, iface)
		})
	}
}
