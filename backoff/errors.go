package backoff

// stringerr is a constant error type; errorsx can't be used here since it
// depends on this package.
type stringerr string

func (t stringerr) Error() string {
	return string(t)
}

const (
	ErrStopAttempts = stringerr("attempts stopped")
)
