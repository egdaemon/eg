// Package envx provides utility functions for extracting information from environment variables
package envx

import (
	"bufio"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
)

func Print(environ []string) {
	log.Println("--------- PRINT ENVIRON INITIATED ---------")
	defer log.Println("--------- PRINT ENVIRON COMPLETED ---------")
	for _, s := range environ {
		log.Println(s)
	}
}

// Int retrieve a integer flag from the environment, checks each key in order
// first to parse successfully is returned.
func Int(fallback int, keys ...string) int {
	return envval(fallback, func(s string) (int, error) {
		decoded, err := strconv.ParseInt(s, 10, 64)
		return int(decoded), errorsx.Wrapf(err, "integer '%s' is invalid", s)
	}, keys...)
}

func Float64(fallback float64, keys ...string) float64 {
	return envval(fallback, func(s string) (float64, error) {
		decoded, err := strconv.ParseFloat(s, 64)
		return float64(decoded), errorsx.Wrapf(err, "float64 '%s' is invalid", s)
	}, keys...)
}

// Boolean retrieve a boolean flag from the environment, checks each key in order
// first to parse successfully is returned.
func Boolean(fallback bool, keys ...string) bool {
	return envval(fallback, func(s string) (bool, error) {
		decoded, err := strconv.ParseBool(s)
		return decoded, errorsx.Wrapf(err, "boolean '%s' is invalid", s)
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

// Duration retrieves a time.Duration from the environment, checks each key in order
// first successful parse to a duration is returned.
func Duration(fallback time.Duration, keys ...string) time.Duration {
	return envval(fallback, func(s string) (time.Duration, error) {
		decoded, err := time.ParseDuration(s)
		return decoded, errorsx.Wrapf(err, "time.Duration '%s' is invalid", s)
	}, keys...)
}

// Hex read value as a hex encoded string.
func Hex(fallback []byte, keys ...string) []byte {
	return envval(fallback, func(s string) ([]byte, error) {
		decoded, err := hex.DecodeString(s)
		return decoded, errorsx.Wrapf(err, "invalid hex encoded data '%s'", s)
	}, keys...)
}

// Base64 read value as a base64 encoded string
func Base64(fallback []byte, keys ...string) []byte {
	enc := base64.RawStdEncoding.WithPadding('=')
	return envval(fallback, func(s string) ([]byte, error) {
		decoded, err := enc.DecodeString(s)
		return decoded, errorsx.Wrapf(err, "invalid base64 encoded data '%s'", s)
	}, keys...)
}

func URL(fallback string, keys ...string) *url.URL {
	var (
		err    error
		parsed *url.URL
	)

	if parsed, err = url.Parse(fallback); err != nil {
		panic(errorsx.Wrap(err, "must provide a valid fallback url"))
	}

	return envval(parsed, func(s string) (*url.URL, error) {
		decoded, err := url.Parse(s)
		return decoded, errorsx.WithStack(err)
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

func Debug(envs ...string) {
	errorsx.MaybeLog(log.Output(2, fmt.Sprintln("DEBUG ENVIRONMENT INITIATED")))
	defer func() { errorsx.MaybeLog(log.Output(3, "DEBUG ENVIRONMENT COMPLETED")) }()
	for _, e := range envs {
		errorsx.MaybeLog(log.Output(2, fmt.Sprintln(e)))
	}
}

func FromReader(r io.Reader) (environ []string, err error) {
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		environ = append(environ, scanner.Text())
	}

	return environ, nil
}

func FromPath(n string) (environ []string, err error) {
	env, err := os.Open(n)
	if os.IsNotExist(err) {
		return environ, nil
	}
	if err != nil {
		return nil, err
	}
	defer env.Close()

	return FromReader(env)
}

type builder struct {
	environ []string
	failed  error
}

func (t builder) Environ() ([]string, error) {
	return t.environ, t.failed
}

func (t builder) FromPath(n string) builder {
	tmp, err := FromPath(n)
	t.environ = append(t.environ, tmp...)
	t.failed = errors.Join(t.failed, err)
	return t
}

func Build() builder {
	return builder{}
}
