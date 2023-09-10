package ffiegcontainer

//export github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Pull
func Pull(name string, args []string) uint32

//export github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Build
func Build(name, definition string, args []string) uint32

//export github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Run
func Run(name, modulepath string, cmd []string, args []string) uint32

//export github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Module
func Module(name, modulepath string, args []string) uint32
