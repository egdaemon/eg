package app

import (
	"fmt"

	"example.com/callers/lib"
)

func Run() {
	if v := lib.Target(); v != "" {
		fmt.Println(v)
	}
}
