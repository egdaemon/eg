// Package envx provides utility functions for extracting information from environment variables
package envx

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Boolean retrieve a boolean flag from the environment, checks each key in order
// first to parse successfully is returned.
func Boolean(fallback bool, keys ...string) bool {
	for _, k := range keys {
		if b, err := strconv.ParseBool(os.Getenv(k)); err == nil {
			return b
		}
	}

	return fallback
}

// String retrieve a string value from the environment, checks each key in order
// first string found is returned.
func String(fallback string, keys ...string) string {
	for _, k := range keys {
		s := strings.TrimSpace(os.Getenv(k))
		if s != "" {
			return s
		}
	}

	return fallback
}

// Duration retrieves a time.Duration from the environment, checks each key in order
// first successful parse to a duration is returned.
func Duration(fallback time.Duration, keys ...string) time.Duration {
	for _, k := range keys {
		s := strings.TrimSpace(os.Getenv(k))
		if s == "" {
			continue
		}

		if d, err := time.ParseDuration(s); err == nil {
			return d
		} else {
			log.Println(errors.Wrapf(err, "unable to parse time.Duration from %s", s))
		}
	}

	return fallback
}
