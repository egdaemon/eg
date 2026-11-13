package ffierrors

const (
	ErrNotImplemented = 999
	ErrUnrecoverable  = 1000
	ErrNotFound       = 1001 // the host has nothing by the name the guest asked for.
	ErrPending        = 1002 // the operation the guest is waiting on has not finished.
	ErrBufferTooSmall = 1003 // the buffer the guest provided cannot hold the result, the required length was written.
)
