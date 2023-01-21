package ffiegcontainer

//export github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Build
func Build(name, definition string) int

//export github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegcontainer.Run
func Run(name, modulepath string) int
