package computesess

// Running reports whether the session has not been stopped. a session that is running has no time it stopped,
// which the generated scanner reads as the zero time.
func (t Session) Running() bool {
	return t.StoppedAt == nil || t.StoppedAt.IsZero()
}
