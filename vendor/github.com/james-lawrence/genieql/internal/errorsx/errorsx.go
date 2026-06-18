package errorsx

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"
)

func Wrap(err error, message string) error {
	return errors.Wrap(err, message)
}

func Wrapf(err error, format string, args ...interface{}) error {
	return errors.Wrapf(err, format, args...)
}

func Errorf(format string, args ...interface{}) error {
	return errors.Errorf(format, args...)
}

func New(msg string) error {
	return errors.New(msg)
}

func WithStack(err error) error {
	return errors.WithStack(err)
}

// Compact returns the first error in the set, if any.
func Compact(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

func Ignore(err error, ignore ...error) error {
	for _, i := range ignore {
		if errors.Is(err, i) {
			return nil
		}
	}

	return err
}

// NewErrRatelimit creates a new rate limit error with the provided backoff.
func NewErrRatelimit(err error, backoff time.Duration) ErrRatelimit {
	return ratelimit{error: err, backoff: backoff}
}

// ErrRatelimit ...
type ErrRatelimit interface {
	error
	Backoff() time.Duration
}

type ratelimit struct {
	backoff time.Duration
	error
}

func (t ratelimit) Backoff() time.Duration {
	return t.backoff
}

// String representing an error, useful for declaring string constants as errors.
type String string

func (t String) Error() string {
	return string(t)
}

// StackChecksum computes a checksum of the given error
// using its stack trace.
func StackChecksum(err error) string {
	type stackTracer interface {
		StackTrace() errors.StackTrace
	}

	hash := md5.New()
	if err != nil {
		if failure, ok := err.(stackTracer); ok {
			for _, frame := range failure.StackTrace() {
				hash.Write([]byte(fmt.Sprint(frame)))
			}
		}
	}

	sum := hash.Sum(nil)
	return hex.EncodeToString(sum[:])
}

func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}

func Zero[T any](v T, err error) (zero T) {
	if err != nil {
		return zero
	}

	return v
}

// MaybePanic panic when error is seen.
func MaybePanic(err error) {
	if err == nil {
		return
	}

	panic(err)
}

func Log(err error) {
	if err == nil {
		return
	}

	if cause := log.Output(2, fmt.Sprintln(err)); cause != nil {
		panic(cause)
	}
}

type Unrecoverable struct {
	cause error
}

func (t Unrecoverable) Unrecoverable() {}

func (t Unrecoverable) Unwrap() error {
	return t.cause
}

func (t Unrecoverable) Error() string {
	return t.cause.Error()
}

func (t Unrecoverable) Is(target error) bool {
	type unrecoverable interface {
		Unrecoverable()
	}

	_, ok := target.(unrecoverable)
	return ok
}

func (t Unrecoverable) As(target any) bool {
	type unrecoverable interface {
		Unrecoverable()
	}

	if x, ok := target.(*unrecoverable); ok {
		*x = t
		return ok
	}

	return false
}

func NewUnrecoverable(err error) error {
	return Unrecoverable{
		cause: err,
	}
}
