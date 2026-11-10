// guest used by the mount tests: performs a single filesystem operation and exits non-zero when it fails.
package main

import (
	"log"
	"os"
)

// os.Args[1] selects the operation, the remaining arguments are its paths:
//
//	readdir path        - lists the directory.
//	mkdirall path       - creates the directory and its parents.
//	rename from to      - renames from to to.
//	write path          - writes a file.
//	read path           - reads a file.
func main() {
	var (
		err error
	)

	switch os.Args[1] {
	case "readdir":
		_, err = os.ReadDir(os.Args[2])
	case "mkdirall":
		err = os.MkdirAll(os.Args[2], 0700)
	case "rename":
		err = os.Rename(os.Args[2], os.Args[3])
	case "write":
		err = os.WriteFile(os.Args[2], []byte("hello"), 0600)
	case "read":
		_, err = os.ReadFile(os.Args[2])
	default:
		log.Fatalln("unknown operation", os.Args[1])
	}

	if err != nil {
		log.Fatalln(err)
	}
}
