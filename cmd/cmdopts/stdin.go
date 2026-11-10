package cmdopts

import "io"

// Stdin is the kong binding commands accept to read their input, allowing tests
// to inject a reader instead of touching the process-global os.Stdin.
type Stdin interface {
	io.Reader
}
