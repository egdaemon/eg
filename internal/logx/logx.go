package logx

import (
	"fmt"
	"log"
)

// Verbose include stack trace
func Verbose(err error) error {
	if err != nil {
		log.Output(2, fmt.Sprintf("%+v\n", err))
	}

	return err
}
