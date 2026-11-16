package execx

type args []string

// Args starts the arguments of a command, spread the result into the arguments of a call.
//
//	execx.String(ctx, "systemctl", execx.Args("reset-failed", "--wait").Append(units...)...)
func Args(a ...string) args {
	return a
}

// Append returns the arguments followed by more, the arguments it was called on are left as they were.
func (t args) Append(more ...string) args {
	return append(t[:len(t):len(t)], more...)
}
