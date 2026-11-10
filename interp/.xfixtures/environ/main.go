// guest used by the module environment tests: prints its environment, one entry per line.
package main

import (
	"fmt"
	"os"
)

func main() {
	for _, e := range os.Environ() {
		fmt.Println(e)
	}
}
