package ipx

import (
	"net"
)

// DefaultIfBlank uses the default value if the provided string is blank.
func DefaultIfBlank(s, defaultValue net.IP) net.IP {
	if s == nil {
		return defaultValue
	}

	return s
}

// First get the first value from the array.
func First(values ...net.IP) net.IP {
	if len(values) == 0 {
		return nil
	}

	return values[0]
}
