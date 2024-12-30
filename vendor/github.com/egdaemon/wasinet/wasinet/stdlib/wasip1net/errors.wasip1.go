//go:build wasip1

package wasip1net

import (
	"context"
	"errors"
)

// Various errors contained in OpError.
var (
	// For connection setup operations.
	errNoSuitableAddress = errors.New("no suitable address found")

	// For connection setup and write operations.
	errMissingAddress = errors.New("missing address")

	// For both read and write operations.
	errCanceled         = canceledError{}
	ErrWriteToConnected = errors.New("use of WriteTo with pre-connected connection")
)

// canceledError lets us return the same error string we have always
// returned, while still being Is context.Canceled.
type canceledError struct{}

func (canceledError) Error() string { return "operation was canceled" }

func (canceledError) Is(err error) bool { return err == context.Canceled }
