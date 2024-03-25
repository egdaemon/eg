package errorsx

import (
	"fmt"
	"log"
	"time"

	"github.com/pkg/errors"
)

// Compact returns the first error in the set, if any.
func Compact(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

func MaybeLog(err error) {
	if err == nil {
		return
	}

	if cause := log.Output(2, fmt.Sprintln(err)); cause != nil {
		panic(cause)
	}
}

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

func Wrap(err error, m string) error {
	return errors.Wrap(err, m)
}

func Wrapf(err error, format string, args ...interface{}) error {
	return errors.Wrapf(err, format, args...)
}

func WithStack(err error) error {
	return errors.WithStack(err)
}

func Ignore(err error, ignore ...error) error {
	for _, i := range ignore {
		if errors.Is(err, i) {
			return nil
		}
	}

	return err
}

// CompactMonad an error that collects and returns the first error encountered.
type CompactMonad struct {
	cause error
}

func (t CompactMonad) Error() string { return t.cause.Error() }

// Cause implement errors.Cause interface.
func (t CompactMonad) Cause() error { return t.cause }

// Compact returns a monad holding the first error
// encountered.
func (t CompactMonad) Compact(errs ...error) CompactMonad {
	if t.cause != nil {
		return t
	}

	return CompactMonad{cause: Compact(errs...)}
}

// String useful wrapper for string constants as errors.
type String string

func (t String) Error() string {
	return string(t)
}

// Timeout error.
type Timeout interface {
	Timedout() time.Duration
}

// Timedout represents a timeout.
func Timedout(cause error, d time.Duration) error {
	if cause == nil {
		return nil
	}

	return timeout{
		error: cause,
		d:     d,
	}
}

type timeout struct {
	error
	d time.Duration
}

func (t timeout) Timedout() time.Duration {
	return t.d
}

// Notification presents an error that will be displayed to the user
// to provide notifications.
func Notification(err error) error {
	return notification{
		error: err,
	}
}

type notification struct {
	error
}

func (t notification) Notification() {}
func (t notification) Unwrap() error {
	return t.error
}
func (t notification) Cause() error {
	return t.error
}

// UserFriendly represents an error that will be displayed to users.
func UserFriendly(err error) error {
	return userfriendly{
		error: err,
	}
}

type userfriendly struct {
	error
}

// user friendly error
func (t userfriendly) UserFriendly() {}
func (t userfriendly) Unwrap() error {
	return t.error
}
func (t userfriendly) Cause() error {
	return t.error
}

// Mark an error as temporary
func NewTemporary(err error) error {
	return Temporary{
		error: err,
	}
}

type Temporary struct {
	error
}

func (t Temporary) Temporary() bool {
	return true
}

func (t Temporary) Unwrap() error {
	return t.error
}
func (t Temporary) Cause() error {
	return t.error
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
