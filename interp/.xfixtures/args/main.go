// guest used by the module argument tests: prints its arguments, one quoted entry per line.
package main

import (
	"fmt"
	"os"
)

func main() {
	for _, a := range os.Args {
		fmt.Printf("%q\n", a)
	}
}
