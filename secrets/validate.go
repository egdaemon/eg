package secrets

import (
	"fmt"
	"net/url"
)

// ValidateRemote ensures the uris are references to hosted secret managers
// (gcpsm, awssm) that are safe to accept from untrusted callers, e.g. scheduled
// workloads. schemes that read from the local filesystem or embed a passphrase
// (file, chachasm) are rejected.
func ValidateRemote(uris ...string) error {
	for _, raw := range uris {
		u, err := url.Parse(raw)
		if err != nil {
			return fmt.Errorf("invalid secret uri: %w", err)
		}

		switch u.Scheme {
		case "gcpsm", "awssm":
		default:
			return fmt.Errorf("unsupported secret uri scheme %q: only gcpsm and awssm are allowed", u.Scheme)
		}

		if u.User != nil {
			return fmt.Errorf("secret uri must not contain user information")
		}

		if u.Host == "" && u.Path == "" {
			return fmt.Errorf("secret uri must reference a secret")
		}
	}

	return nil
}
