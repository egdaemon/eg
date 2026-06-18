package duckproxy

// Logger is the minimal logging interface duckproxy uses to report
// per-connection errors that are not surfaced to the client (e.g. a write
// failing after the client has already disconnected).
type Logger interface {
	Printf(format string, args ...any)
}

type noopLogger struct{}

func (noopLogger) Printf(string, ...any) {}
