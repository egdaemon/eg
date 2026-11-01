// Package envx provides helpers for reading configuration from the environment.
package envx

import (
	"log"
	"os"
	"strings"
	"time"
)

// Duration retrieves a time.Duration from the environment, checks each key in order
// first successful parse to a duration is returned, otherwise the fallback.
func Duration(fallback time.Duration, keys ...string) time.Duration {
	for _, k := range keys {
		s := strings.TrimSpace(os.Getenv(k))
		if s == "" {
			continue
		}

		decoded, err := time.ParseDuration(s)
		if err != nil {
			log.Printf("%s stored an invalid time.Duration '%s': %v\n", k, s, err)
			continue
		}

		return decoded
	}

	return fallback
}
