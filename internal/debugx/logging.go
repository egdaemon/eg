// +build debug.enabled

package debugx

import (
	"fmt"
	"log"
)

// Println prints to log.Println
func Println(v ...interface{}) {
	log.Output(2, fmt.Sprintln(v...))
}

// Printf prints to log.Printf
func Printf(format string, v ...interface{}) {
	log.Output(2, fmt.Sprintf(format, v...))
}

// Print prints to log.Print
func Print(v ...interface{}) {
	log.Output(2, fmt.Sprint(v...))
}
