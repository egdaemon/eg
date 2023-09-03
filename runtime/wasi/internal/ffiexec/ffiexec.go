package ffiexec

//export github.com/james-lawrence/eg/runtime/wasi/runtime/ffiexec.Command
func Command(deadline int64, command string, args []string) uint32
