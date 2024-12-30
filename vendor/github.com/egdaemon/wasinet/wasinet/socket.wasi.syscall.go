package wasinet

// ported constants that dont exist in syscall for the wasi environment.
const (
	SOL_SOCKET   = 0x1
	SO_REUSEADDR = 0x2
	SO_BROADCAST = 0x6
	SO_RCVTIMEO  = 20
	SO_SNDTIMEO  = 21
)
