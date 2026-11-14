package servesessions

// Running reports whether the session has not been stopped, a stopped session has the time it stopped.
func (t Session) Running() bool {
	return t.StoppedAt.IsZero()
}
