package ffiegmodule

// //export github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegmodule.ImportAlias
// func ImportAlias(pkg string) string

// //export github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegmodule.Build
// func Build(mainfn string) int

//export github.com/james-lawrence/eg/runtime/wasi/runtime/ffiegmodule.Build
func Build(main ...string) int {
	return -1
}
