// guest used by the cell tests: copies the image's /etc/hostname into the working directory.
package main

import (
	"log"
	"os"

	"github.com/egdaemon/eg"
)

func main() {
	b, err := os.ReadFile("/etc/hostname")
	if err != nil {
		log.Fatalln(err)
	}

	if err = os.WriteFile(eg.DefaultWorkingDirectory("hostname"), b, 0600); err != nil {
		log.Fatalln(err)
	}
}
