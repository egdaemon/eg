// Package envvars provides tests for nesting behaviors of environment variables for modules.
package main

import (
	"log"
)

func main() {
	log.SetFlags(log.Lshortfile | log.LUTC | log.Ldate)
	log.Println("hello world")
}
