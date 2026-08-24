// Package errorsx provides small utilities extending the standard errors package.
package errorsx

import (
	"errors"
	"fmt"
	"log"
)

func Log(err error) {
	if err == nil {
		return
	}

	if cause := log.Output(2, fmt.Sprintln(err)); cause != nil {
		panic(cause)
	}
}

// Ignore returns nil if err matches any of the targets, err otherwise.
func Ignore(err error, targets ...error) error {
	for _, target := range targets {
		if errors.Is(err, target) {
			return nil
		}
	}

	return err
}
