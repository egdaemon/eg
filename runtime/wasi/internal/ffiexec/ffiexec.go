package ffiexec

//export github.com/james-lawrence/eg/runtime/wasi/runtime/ffiexec.Command
func Command(deadline int64, dir string, command string, args []string) uint32
