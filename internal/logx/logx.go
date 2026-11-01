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

// Conditional is a logger that drops messages unless it is enabled.
type Conditional struct {
	enabled bool
}

// If creates a logger that only outputs when enabled is true, e.g.)
// logx.If(envx.Boolean(false, "EG_LOGS_INFO")).Printf("...").
func If(enabled bool) Conditional {
	return Conditional{enabled: enabled}
}

// Printf logs the message when enabled.
func (t Conditional) Printf(format string, v ...any) {
	if !t.enabled {
		return
	}

	log.Printf(format, v...)
}
