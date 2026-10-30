package sample

import (
	"fmt"
	"strings"
)

func Shout(name string) string {
	return strings.ToUpper(fmt.Sprintf("hello %s", name))
}

func quiet(name string) string {
	return strings.ToLower(name)
}
