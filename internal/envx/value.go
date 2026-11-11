package envx

import (
	"encoding/base64"
	"encoding/hex"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/egdaemon/eg/internal/errorsx"
)

// Primitive are the types that can be read from the environment by New.
type Primitive interface {
	~string | ~int | ~int64 | ~uint | ~uint64 | ~float64 | ~bool
}

// Value is a typed environment lookup with a default. It is an intermediate,
// calling From resolves it to the plain T.
//
//	var port uint = envx.New(uint(8080)).From("PORT")
type Value[T any] struct {
	def   T
	m     func(string) string
	parse func(string) (T, error)
}

// From checks each key in order, the first to parse successfully is returned.
// if none do the default is returned.
func (t Value[T]) From(keys ...string) T {
	return envval(t.def, t.m, t.parse, keys...)
}

// New reads a primitive from the environment. T is inferred from def, note untyped
// constants take their default type (0 is an int), use uint(0) or New[uint](0) otherwise.
func New[T Primitive](def T) Value[T] {
	return NewEnviron(os.Getenv).New(def)
}

// NewBytes reads a byte size, accepts human readable sizes such as '6g', '6GB', or '6GiB'
// in addition to plain integers.
func NewBytes(def uint64) Value[uint64] {
	return NewEnviron(os.Getenv).NewBytes(def)
}

// NewDuration reads a time.Duration.
func NewDuration(def time.Duration) Value[time.Duration] {
	return NewEnviron(os.Getenv).NewDuration(def)
}

// NewHex reads a hex encoded value.
func NewHex(def []byte) Value[[]byte] {
	return NewEnviron(os.Getenv).NewHex(def)
}

// NewBase64 reads a url base64 encoded value.
func NewBase64(def []byte) Value[[]byte] {
	return NewEnviron(os.Getenv).NewBase64(def)
}

// NewURL reads a url, panics if def is not a valid url.
func NewURL(def string) Value[*url.URL] {
	return NewEnviron(os.Getenv).NewURL(def)
}

// New see the package level New.
func (t environ) New[T Primitive](def T) Value[T] {
	return Value[T]{def: def, m: t.m, parse: parsePrimitive[T]}
}

func (t environ) NewBytes(def uint64) Value[uint64] {
	return Value[uint64]{def: def, m: t.m, parse: parseBytes}
}

func (t environ) NewDuration(def time.Duration) Value[time.Duration] {
	return Value[time.Duration]{def: def, m: t.m, parse: parseDuration}
}

func (t environ) NewHex(def []byte) Value[[]byte] {
	return Value[[]byte]{def: def, m: t.m, parse: parseHex}
}

func (t environ) NewBase64(def []byte) Value[[]byte] {
	return Value[[]byte]{def: def, m: t.m, parse: parseBase64(base64.URLEncoding)}
}

func (t environ) NewURL(def string) Value[*url.URL] {
	parsed, err := url.Parse(def)
	if err != nil {
		panic(errorsx.Wrap(err, "must provide a valid fallback url"))
	}

	return Value[*url.URL]{def: parsed, m: t.m, parse: parseURL}
}

func parsePrimitive[T Primitive](s string) (T, error) {
	var out T

	// reflect so that ~ types (type Port uint16) work.
	rv := reflect.ValueOf(&out).Elem()
	switch rv.Kind() {
	case reflect.String:
		rv.SetString(s)
	case reflect.Int, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return out, errorsx.Wrapf(err, "integer '%s' is invalid", s)
		}
		rv.SetInt(n)
	case reflect.Uint, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return out, errorsx.Wrapf(err, "unsigned integer '%s' is invalid", s)
		}
		rv.SetUint(n)
	case reflect.Float64:
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return out, errorsx.Wrapf(err, "float '%s' is invalid", s)
		}
		rv.SetFloat(n)
	case reflect.Bool:
		n, err := strconv.ParseBool(s)
		if err != nil {
			return out, errorsx.Wrapf(err, "boolean '%s' is invalid", s)
		}
		rv.SetBool(n)
	}

	return out, nil
}

func parseBytes(s string) (uint64, error) {
	decoded, err := humanize.ParseBytes(s)
	return decoded, errorsx.Wrapf(err, "bytes '%s' is invalid", s)
}

func parseDuration(s string) (time.Duration, error) {
	decoded, err := time.ParseDuration(s)
	return decoded, errorsx.Wrapf(err, "time.Duration '%s' is invalid", s)
}

func parseHex(s string) ([]byte, error) {
	decoded, err := hex.DecodeString(s)
	return decoded, errorsx.Wrapf(err, "invalid hex encoded data '%s'", s)
}

func parseURL(s string) (*url.URL, error) {
	decoded, err := url.Parse(s)
	return decoded, errorsx.WithStack(err)
}

// parseBase64 decodes the value with the first encoding that succeeds, failing with the error of the first encoding.
func parseBase64(encs ...*base64.Encoding) func(string) ([]byte, error) {
	return func(s string) ([]byte, error) {
		var first error
		for _, enc := range encs {
			decoded, err := enc.DecodeString(s)
			if err == nil {
				return decoded, nil
			}

			if first == nil {
				first = err
			}
		}

		return nil, errorsx.Wrapf(first, "invalid base64 encoded data '%s'", s)
	}
}

// Int retrieve a integer flag from the environment, checks each key in order
// first to parse successfully is returned.
func (t environ) Int(fallback int, keys ...string) int {
	return t.New(fallback).From(keys...)
}

// retrieve a uint64 flag from the environment, checks each key in order
// first to parse successfully is returned.
func (t environ) Uint64(fallback uint64, keys ...string) uint64 {
	return t.New(fallback).From(keys...)
}

// Bytes retrieve a byte size flag from the environment, checks each key in order
// first to parse successfully is returned. accepts human readable byte sizes
// such as '6g', '6GB', or '6GiB' in addition to plain integers.
func (t environ) Bytes(fallback uint64, keys ...string) uint64 {
	return t.NewBytes(fallback).From(keys...)
}

func (t environ) Float64(fallback float64, keys ...string) float64 {
	return t.New(fallback).From(keys...)
}

// Boolean retrieve a boolean flag from the environment, checks each key in order
// first to parse successfully is returned.
func (t environ) Boolean(fallback bool, keys ...string) bool {
	return t.New(fallback).From(keys...)
}

// String retrieve a string value from the environment, checks each key in order
// first string found is returned.
func (t environ) String(fallback string, keys ...string) string {
	return t.New(fallback).From(keys...)
}

// Duration retrieves a time.Duration from the environment, checks each key in order
// first successful parse to a duration is returned.
func (t environ) Duration(fallback time.Duration, keys ...string) time.Duration {
	return t.NewDuration(fallback).From(keys...)
}

// Hex read value as a hex encoded string.
func (t environ) Hex(fallback []byte, keys ...string) []byte {
	return t.NewHex(fallback).From(keys...)
}

// Base64 read value as a base64 encoded string, the standard alphabet is attempted first and then the url alphabet.
func (t environ) Base64(fallback []byte, keys ...string) []byte {
	return Value[[]byte]{def: fallback, m: t.m, parse: parseBase64(base64.RawStdEncoding.WithPadding('='), base64.URLEncoding)}.From(keys...)
}

// Base64URL read value as a url base64 encoded string
func (t environ) Base64URL(fallback []byte, keys ...string) []byte {
	return t.NewBase64(fallback).From(keys...)
}

func (t environ) URL(fallback string, keys ...string) *url.URL {
	return t.NewURL(fallback).From(keys...)
}
