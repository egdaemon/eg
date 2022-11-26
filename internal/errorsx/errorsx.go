package errorsx

import (
	"errors"
	"fmt"
	"log"
	"time"
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

func MaybeLog(err error) error {
	if err == nil {
		return err
	}

	log.Output(1, fmt.Sprintln(err))
	return err
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
