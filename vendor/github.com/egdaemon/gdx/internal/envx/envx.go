// Package envx provides utility functions for extracting information from environment variables.
package envx

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Int retrieve a integer flag from the environment, checks each key in order
// first to parse successfully is returned.
func Int[T int | int64](fallback T, keys ...string) T {
	return envval(fallback, func(s string) (T, error) {
		decoded, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("integer '%s' is invalid: %w", s, err)
		}
		return T(decoded), nil
	}, keys...)
}

// Boolean retrieve a boolean flag from the environment, checks each key in order
// first to parse successfully is returned.
func Boolean(fallback bool, keys ...string) bool {
	return envval(fallback, func(s string) (bool, error) {
		decoded, err := strconv.ParseBool(s)
		if err != nil {
			return false, fmt.Errorf("boolean '%s' is invalid: %w", s, err)
		}
		return decoded, nil
	}, keys...)
}

// Float64 retrieve a float64 flag from the environment, checks each key in order
// first to parse successfully is returned.
func Float64(fallback float64, keys ...string) float64 {
	return envval(fallback, func(s string) (float64, error) {
		decoded, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("float64 '%s' is invalid: %w", s, err)
		}
		return decoded, nil
	}, keys...)
}

// String retrieve a string value from the environment, checks each key in order
// first string found is returned.
func String(fallback string, keys ...string) string {
	return envval(fallback, func(s string) (string, error) {
		// we'll never receive an empty string because envval skips empty strings.
		return s, nil
	}, keys...)
}

// Strings retrieve a string array separated by , value from the environment, checks each key in order
// first string found is returned.
func Strings(fallback []string, keys ...string) []string {
	return envval(fallback, func(s string) ([]string, error) {
		// we'll never receive an empty string because envval skips empty strings.
		return strings.Split(s, ","), nil
	}, keys...)
}

// Duration retrieves a time.Duration from the environment, checks each key in order
// first successful parse to a duration is returned.
func Duration(fallback time.Duration, keys ...string) time.Duration {
	return envval(fallback, func(s string) (time.Duration, error) {
		decoded, err := time.ParseDuration(s)
		if err != nil {
			return 0, fmt.Errorf("time.Duration '%s' is invalid: %w", s, err)
		}
		return decoded, nil
	}, keys...)
}

// BytesFile treats the value in the provided environment keys as a file path.
func BytesFile(fallback []byte, keys ...string) []byte {
	return envval(fallback, func(s string) ([]byte, error) {
		decoded, err := os.ReadFile(s)
		if err != nil {
			return nil, fmt.Errorf("file path '%s' was inaccessible: %w", s, err)
		}
		return decoded, nil
	}, keys...)
}

// BytesHex read value as a hex encoded string.
func BytesHex(fallback []byte, keys ...string) []byte {
	return envval(fallback, func(s string) ([]byte, error) {
		decoded, err := hex.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid hex encoded data '%s': %w", s, err)
		}
		return decoded, nil
	}, keys...)
}

// BytesB64 read value as a base64 encoded string
func BytesB64(fallback []byte, keys ...string) []byte {
	enc := base64.RawStdEncoding.WithPadding('=')
	return envval(fallback, func(s string) ([]byte, error) {
		decoded, err := enc.DecodeString(s)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 encoded data '%s': %w", s, err)
		}
		return decoded, nil
	}, keys...)
}

func URL(fallback string, keys ...string) *url.URL {
	parsed, err := url.Parse(fallback)
	if err != nil {
		panic(fmt.Errorf("must provide a valid fallback url: %w", err))
	}

	return envval(parsed, func(s string) (*url.URL, error) {
		return url.Parse(s)
	}, keys...)
}

func envval[T any](fallback T, parse func(string) (T, error), keys ...string) T {
	for _, k := range keys {
		s := strings.TrimSpace(os.Getenv(k))
		if s == "" {
			continue
		}

		decoded, err := parse(s)
		if err != nil {
			log.Printf("%s stored an invalid value %v\n", k, err)
			continue
		}

		return decoded
	}

	return fallback
}
