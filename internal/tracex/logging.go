//go:build !trace.disabled
// +build !trace.disabled

package tracex

import (
	"fmt"
	"io"
	"log"
)

var std = log.New(io.Discard, "", log.LstdFlags)

// SetOutput sets the output destination for the debug logger.
func SetOutput(w io.Writer) {
	std.SetOutput(w)
}

// Flags returns the output flags for the standard logger.
// The flag bits are [Ldate], [Ltime], and so on.
func Flags() int {
	return std.Flags()
}

// SetFlags sets the output flags for the standard logger.
// The flag bits are [Ldate], [Ltime], and so on.
func SetFlags(flag int) {
	std.SetFlags(flag)
}

// Println prints to log.Println
func Println(v ...interface{}) {
	_ = std.Output(2, fmt.Sprintln(v...))
}

// Printf prints to log.Printf
func Printf(format string, v ...interface{}) {
	_ = std.Output(2, fmt.Sprintf(format, v...))
}

// Print prints to log.Print
func Print(v ...interface{}) {
	_ = std.Output(2, fmt.Sprint(v...))
}
