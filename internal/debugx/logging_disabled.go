//go:build debug.disabled
// +build debug.disabled

package debugx

// Println noop. should get optimized out by compiler.
func Println(v ...interface{}) {}

// Printf noop. should get optimized out by compiler.
func Printf(format string, v ...interface{}) {}

// Print noop. should get optimized out by compiler.
func Print(v ...interface{}) {}
