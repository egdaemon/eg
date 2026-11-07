// reactor used by the loader tests: built with -buildmode=c-shared, its package initialization only runs
// through _initialize.
package main

var initialized int32

func init() {
	initialized = 1
}

// Initialized reports whether the reactor's _initialize ran.
//
//go:wasmexport initialized
func Initialized() int32 {
	return initialized
}

func main() {}
