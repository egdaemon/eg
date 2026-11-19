package cmdopts

import (
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

// ParseMount parses a mount specification of the form source:destination[:mode[:user[:group]]].
// mode is octal, e.g.) 0700, and user and group may be blank. the access is decided by the flag providing
// the specification, not the specification itself.
func ParseMount(spec string, access Access) (m Mount, err error) {
	parts := strings.Split(spec, ":")
	if len(parts) < 2 || len(parts) > 5 {
		return m, fmt.Errorf("invalid mount %q, expected source:destination[:mode[:user[:group]]]", spec)
	}

	m.Source, m.Destination, m.Access = parts[0], parts[1], access

	if strings.TrimSpace(m.Source) == "" {
		return m, fmt.Errorf("invalid mount %q, the source is required", spec)
	}

	if strings.TrimSpace(m.Destination) == "" {
		return m, fmt.Errorf("invalid mount %q, the destination is required", spec)
	}

	if len(parts) > 2 && parts[2] != "" {
		mode, err := strconv.ParseUint(parts[2], 8, 32)
		if err != nil {
			return m, fmt.Errorf("invalid mount %q, the mode must be octal, e.g.) 0700: %w", spec, err)
		}

		m.Mode = fs.FileMode(mode).Perm()
	}

	if len(parts) > 3 {
		m.User = parts[3]
	}

	if len(parts) > 4 {
		m.Group = parts[4]
	}

	return m, nil
}
