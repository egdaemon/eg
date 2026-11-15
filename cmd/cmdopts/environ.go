package cmdopts

import "os"

// Environ is the kong binding commands accept to look up environment variables, allowing tests to inject
// them instead of touching the process environment.
type Environ func(string) string

// DefaultEnviron looks up the process environment.
func DefaultEnviron(name string) string {
	return os.Getenv(name)
}
