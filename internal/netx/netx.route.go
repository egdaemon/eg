package netx

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/egdaemon/eg/internal/errorsx"
)

const rtfUp = 0x1

// DefaultRouteInterface returns the name of the interface the host's default route uses.
// linux only, it reads /proc/net/route.
func DefaultRouteInterface() (string, error) {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return "", errorsx.Wrap(err, "unable to read the routing table")
	}
	defer f.Close()

	return parseDefaultRouteInterface(f)
}

// parseDefaultRouteInterface finds the default route (destination and mask both 0.0.0.0) that is up
// with the lowest metric in the /proc/net/route format.
func parseDefaultRouteInterface(r io.Reader) (_ string, err error) {
	var (
		iface  string
		metric uint64
	)

	scanner := bufio.NewScanner(r)
	scanner.Scan() // header.
	for scanner.Scan() {
		// Iface Destination Gateway Flags RefCnt Use Metric Mask ...
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 || fields[1] != "00000000" || fields[7] != "00000000" {
			continue
		}

		if flags, err := strconv.ParseUint(fields[3], 16, 32); err != nil || flags&rtfUp == 0 {
			continue
		}

		m, err := strconv.ParseUint(fields[6], 10, 32)
		if err != nil {
			continue
		}

		if iface == "" || m < metric {
			iface, metric = fields[0], m
		}
	}

	if err = scanner.Err(); err != nil {
		return "", errorsx.Wrap(err, "unable to read the routing table")
	}

	if iface == "" {
		return "", errorsx.String("no default route")
	}

	return iface, nil
}
