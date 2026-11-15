// Package egenvx reads typed values from the environment. a Reader is built with its default and resolved against
// the keys that may hold it:
//
//	var port uint = egenvx.New(uint(8080)).From("PORT")
//
// readers resolve when From is called, and From has the same shape as Exists, so they compose with Deferred to read the
// environment when a workload step runs instead of when its graph is built.
package egenvx

import (
	"context"
	"net/url"
	"time"

	"github.com/egdaemon/eg/internal/envx"
)

// Primitive are the types that can be read from the environment by New.
type Primitive = envx.Primitive

// Reader is a typed environment lookup with a default, From resolves it to the plain T.
type Reader[T any] = envx.Reader[T]

// New reads a primitive from the environment. T is inferred from def, note untyped
// constants take their default type (0 is an int), use uint(0) or New[uint](0) otherwise.
func New[T Primitive](def T) Reader[T] {
	return envx.New(def)
}

// NewBytes reads a byte size, accepts human readable sizes such as '6g', '6GB', or '6GiB'
// in addition to plain integers.
func NewBytes(def uint64) Reader[uint64] {
	return envx.NewBytes(def)
}

// NewDuration reads a time.Duration.
func NewDuration(def time.Duration) Reader[time.Duration] {
	return envx.NewDuration(def)
}

// NewHex reads a hex encoded value.
func NewHex(def []byte) Reader[[]byte] {
	return envx.NewHex(def)
}

// NewBase64 reads a url base64 encoded value.
func NewBase64(def []byte) Reader[[]byte] {
	return envx.NewBase64(def)
}

// NewURL reads a url, panics if def is not a valid url.
func NewURL(def string) Reader[*url.URL] {
	return envx.NewURL(def)
}

// Exists returns true if any of the keys has a value. like the readers, empty and whitespace only values
// are treated as missing.
func Exists(keys ...string) bool {
	return envx.New("").From(keys...) != ""
}

// Deferred delays reading the environment until the returned function is called, for example
// egenvx.Deferred(egenvx.Exists, "TOKEN") or egenvx.Deferred(egenvx.NewDuration(time.Minute).From, "TTL").
//
// a workload's graph is built before its steps run, so a value read while building it may not match what a step sees
// (for example secrets provided with --secret). read through Deferred inside conditions such as eg.WhenFn to get the
// value at execution time.
func Deferred[T any](read func(keys ...string) T, keys ...string) func(context.Context) T {
	return func(context.Context) T {
		return read(keys...)
	}
}
