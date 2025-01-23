package errorsx

import (
	"errors"
	"fmt"
	"log"
)

// Zero logs that the error occurred but otherwise ignores it.
func Zero[T any](v T, err error) T {
	if err == nil {
		return v
	}

	if cause := log.Output(2, fmt.Sprintln(err)); cause != nil {
		panic(cause)
	}

	return v
}

func Must[T any](v T, err error) T {
	if err == nil {
		return v
	}

	panic(err)
}

func Ignore(err error, ignore ...error) error {
	for _, i := range ignore {
		if errors.Is(err, i) {
			return nil
		}
	}

	return err
}

// Never panic when error is seen.
func Never(err error) {
	if err == nil {
		return
	}

	panic(err)
}
