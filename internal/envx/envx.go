// Package envx provides utility functions for extracting information from environment variables
package envx

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/numericx"
	"github.com/egdaemon/eg/internal/slicesx"
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
	return NewEnviron(os.Getenv).Int(fallback, keys...)
}

// retrieve a uint64 flag from the environment, checks each key in order
// first to parse successfully is returned.
func Uint64(fallback uint64, keys ...string) uint64 {
	return NewEnviron(os.Getenv).Uint64(fallback, keys...)
}

func Float64(fallback float64, keys ...string) float64 {
	return NewEnviron(os.Getenv).Float64(fallback, keys...)
}

// Boolean retrieve a boolean flag from the environment, checks each key in order
// first to parse successfully is returned.
func Boolean(fallback bool, keys ...string) bool {
	return NewEnviron(os.Getenv).Boolean(fallback, keys...)
}

// String retrieve a string value from the environment, checks each key in order
// first string found is returned.
func String(fallback string, keys ...string) string {
	return NewEnviron(os.Getenv).String(fallback, keys...)
}

// Toggle based on environment keys.
func Toggle[T any](off, on T, keys ...string) T {
	if Boolean(false, keys...) {
		return on
	}

	return off
}

// Duration retrieves a time.Duration from the environment, checks each key in order
// first successful parse to a duration is returned.
func Duration(fallback time.Duration, keys ...string) time.Duration {
	return NewEnviron(os.Getenv).Duration(fallback, keys...)
}

// Hex read value as a hex encoded string.
func Hex(fallback []byte, keys ...string) []byte {
	return NewEnviron(os.Getenv).Hex(fallback, keys...)
}

// Base64 read value as a base64 encoded string
func Base64(fallback []byte, keys ...string) []byte {
	return NewEnviron(os.Getenv).Base64(fallback, keys...)
}

func URL(fallback string, keys ...string) *url.URL {
	return NewEnviron(os.Getenv).URL(fallback, keys...)
}

type environ struct {
	m func(string) string
}

func NewEnviron(m func(s string) string) environ {
	return environ{m: m}
}

func NewEnvironFromStrings(environ ...string) environ {
	m := make(map[string]string, len(environ))
	for _, i := range environ {
		if idx := strings.IndexRune(i, '='); idx > -1 {
			m[i[:idx]] = i[idx+1:]
		}
	}

	return NewEnviron(func(k string) string {
		if v, ok := m[k]; ok {
			return v
		}

		return ""
	})
}

func (t environ) Map(s string) string {
	return t.m(s)
}

type expandreader struct {
	s       *bufio.Scanner
	current *bytes.Buffer
	mapping func(string) string
}

func (t expandreader) Read(b []byte) (n int, err error) {
	// If the current internal buffer is empty, we need to scan the next line from the source.
	if t.current.Len() == 0 {
		if !t.s.Scan() { // we're done, return either the scanner's error or io.EOF.
			return 0, errorsx.Compact(t.s.Err(), io.EOF)
		}

		// Get the raw text of the scanned line (bufio.Scanner.Text() strips the delimiter).
		rawLine := t.s.Text()

		// Expand variables in the line using the provided mapping function.
		expanded := os.Expand(rawLine, t.mapping)

		// Append the expanded line to the internal buffer.
		// We explicitly add a newline here because bufio.Scanner.Text() removes it.
		// This ensures line integrity in the output.
		if n, err := t.current.WriteString(expanded + "\n"); err != nil {
			return n, errorsx.Wrap(err, "failed to write to buffer")
		}
	}

	// Read bytes from the internal buffer into the consumer's provided buffer 'b'.
	// This handles the case where 'b' is smaller than the current expanded line.
	return t.current.Read(b)
}

func ExpandInplace(path string, mapping func(string) string) error {
	dst, err := os.CreateTemp(filepath.Dir(path), "expanding.*")
	if err != nil {
		return errorsx.Wrap(err, "unable to create tmp")
	}
	defer os.RemoveAll(dst.Name())
	defer dst.Close()

	src, err := os.Open(path)
	if err != nil {
		return errorsx.Wrapf(err, "unable to open file: %s", path)
	}
	defer src.Close()

	if _, err := io.Copy(dst, ExpandReader(src, mapping)); err != nil {
		return err
	}

	return os.Rename(dst.Name(), path)
}

// ExpandReader
func ExpandReader(r io.Reader, mapping func(string) string) io.Reader {
	return expandreader{
		s:       bufio.NewScanner(r),
		current: bytes.NewBuffer(nil),
		mapping: mapping,
	}
}

// Int retrieve a integer flag from the environment, checks each key in order
// first to parse successfully is returned.
func (t environ) Int(fallback int, keys ...string) int {
	return envval(fallback, t.m, func(s string) (int, error) {
		decoded, err := strconv.ParseInt(s, 10, 64)
		return int(decoded), errorsx.Wrapf(err, "integer '%s' is invalid", s)
	}, keys...)
}

// retrieve a uint64 flag from the environment, checks each key in order
// first to parse successfully is returned.
func (t environ) Uint64(fallback uint64, keys ...string) uint64 {
	return envval(fallback, t.m, func(s string) (uint64, error) {
		decoded, err := strconv.ParseUint(s, 10, 64)
		return decoded, errorsx.Wrapf(err, "uint64 '%s' is invalid", s)
	}, keys...)
}

func (t environ) Float64(fallback float64, keys ...string) float64 {
	return envval(fallback, t.m, func(s string) (float64, error) {
		decoded, err := strconv.ParseFloat(s, 64)
		return float64(decoded), errorsx.Wrapf(err, "float64 '%s' is invalid", s)
	}, keys...)
}

// Boolean retrieve a boolean flag from the environment, checks each key in order
// first to parse successfully is returned.
func (t environ) Boolean(fallback bool, keys ...string) bool {
	return envval(fallback, t.m, func(s string) (bool, error) {
		decoded, err := strconv.ParseBool(s)
		return decoded, errorsx.Wrapf(err, "boolean '%s' is invalid", s)
	}, keys...)
}

// String retrieve a string value from the environment, checks each key in order
// first string found is returned.
func (t environ) String(fallback string, keys ...string) string {
	return envval(fallback, t.m, func(s string) (string, error) {
		// we'll never receive an empty string because envval skips empty strings.
		return s, nil
	}, keys...)
}

// Duration retrieves a time.Duration from the environment, checks each key in order
// first successful parse to a duration is returned.
func (t environ) Duration(fallback time.Duration, keys ...string) time.Duration {
	return envval(fallback, t.m, func(s string) (time.Duration, error) {
		decoded, err := time.ParseDuration(s)
		return decoded, errorsx.Wrapf(err, "time.Duration '%s' is invalid", s)
	}, keys...)
}

// Hex read value as a hex encoded string.
func (t environ) Hex(fallback []byte, keys ...string) []byte {
	return envval(fallback, t.m, func(s string) ([]byte, error) {
		decoded, err := hex.DecodeString(s)
		return decoded, errorsx.Wrapf(err, "invalid hex encoded data '%s'", s)
	}, keys...)
}

// Base64 read value as a base64 encoded string
func (t environ) Base64(fallback []byte, keys ...string) []byte {
	enc := base64.RawStdEncoding.WithPadding('=')
	return envval(fallback, t.m, func(s string) ([]byte, error) {
		decoded, err := enc.DecodeString(s)
		return decoded, errorsx.Wrapf(err, "invalid base64 encoded data '%s'", s)
	}, keys...)
}

func (t environ) URL(fallback string, keys ...string) *url.URL {
	var (
		err    error
		parsed *url.URL
	)

	if parsed, err = url.Parse(fallback); err != nil {
		panic(errorsx.Wrap(err, "must provide a valid fallback url"))
	}

	return envval(parsed, t.m, func(s string) (*url.URL, error) {
		decoded, err := url.Parse(s)
		return decoded, errorsx.WithStack(err)
	}, keys...)
}

func envval[T any](fallback T, m func(string) string, parse func(string) (T, error), keys ...string) T {
	for _, k := range keys {
		s := strings.TrimSpace(m(k))
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

func PrintEnv(envs ...string) string {
	s := fmt.Sprintln("DEBUG ENVIRONMENT INITIATED")
	for _, e := range envs {
		s += fmt.Sprintln(e)
	}
	s += fmt.Sprintln("DEBUG ENVIRONMENT COMPLETED")
	return s
}

func Debug(envs ...string) {
	errorsx.Log(log.Output(2, PrintEnv(envs...)))
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

func Build() *Builder {
	return &Builder{}
}

type Builder struct {
	environ []string
	failed  error
}

func (t Builder) CopyTo(w io.Writer) error {
	if t.failed != nil {
		return t.failed
	}

	for _, e := range t.environ {
		if _, err := fmt.Fprintf(w, "%s\n", e); err != nil {
			return errorsx.Wrapf(err, "unable to write environment variable: %s", e)
		}
	}

	return nil
}

func (t Builder) WriteTo(path string) error {
	var (
		buf bytes.Buffer
	)
	if t.failed != nil {
		return t.failed
	}

	if err := t.CopyTo(&buf); err != nil {
		return err
	}

	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		return err
	}

	return nil
}

func (t Builder) Environ() ([]string, error) {
	return t.environ, t.failed
}

// set a single variable to a value.
func (t *Builder) Var(k, v string) *Builder {
	if encoded := Format(k, v); encoded != "" {
		t.environ = append(t.environ, fmt.Sprintf("%s=%s", k, v))
	}
	return t
}

// extract environment variables from a file on disk.
// missing files are treated as noops.
func (t *Builder) FromPath(p ...string) *Builder {
	for _, n := range p {
		tmp, err := FromPath(n)
		t.environ = append(t.environ, tmp...)
		t.failed = errors.Join(t.failed, err)
	}
	return t
}

// extract environment variables from an io.Reader.
// the format is the standard .env file formats.
func (t *Builder) FromReader(r io.Reader) *Builder {
	tmp, err := FromReader(r)
	t.environ = append(t.environ, tmp...)
	t.failed = errors.Join(t.failed, err)
	return t
}

func (t *Builder) FromEnviron(environ ...string) *Builder {
	t.environ = append(t.environ, environ...)
	return t
}

// extract the key/value pairs from the os.Environ.
// empty keys are passed as k=
func (t *Builder) FromEnv(keys ...string) *Builder {
	vars := make([]string, 0, len(keys))

	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			vars = append(vars, Format(k, v, FormatOptionTransforms(allowAll)))
		}
	}

	t.environ = append(t.environ, vars...)

	return t
}

type formatoption func(*formatopts)
type formatopts struct {
	transformer func(string) string // transforms that result in empty strings are ignored.
}

func allowAll(s string) string {
	return s
}

func ignoreEmptyVariables(s string) string {
	if strings.HasSuffix(s, "=") {
		return ""
	}

	return s
}

func FormatOptionTransforms(transforms ...func(string) string) formatoption {
	combined := func(s string) string {
		for _, trans := range transforms {
			s = trans(s)
		}

		return s
	}

	return func(f *formatopts) {
		f.transformer = combined
	}
}

// set many keys to the same value.
func Vars(v string, keys ...string) (environ []string) {
	environ = make([]string, 0, len(keys))
	for _, k := range keys {
		environ = append(environ, Format(k, v))
	}

	return environ
}

// format a boolean value to true/false strings.
func VarBool(b bool) string {
	return strconv.FormatBool(b)
}

// format a environment variable in k=v.
// - doesn't currently escape values. it may in the future.
// - if the key or value are an empty string it'll return an empty string. it will log if debugging is enabled.
func Format[T ~string](k string, v T, options ...func(*formatopts)) string {
	opts := slicesx.Reduce(&formatopts{
		transformer: ignoreEmptyVariables,
	}, options...)

	evar := strings.TrimSpace(opts.transformer(fmt.Sprintf("%s=%s", k, v)))
	if evar == "" {
		debugx.Println("ignoring variable", k, "empty")
	}

	return fmt.Sprintf("%s=%s", k, v)
}

// see format
func FormatBool(k string, v bool, options ...func(*formatopts)) string {
	return Format(k, strconv.FormatBool(v), options...)
}

// see format
func FormatInt[T numericx.Integer](k string, v T, options ...func(*formatopts)) string {
	return Format(k, fmt.Sprintf("%d", v), options...)
}

// returns the os.Environ or an empty slice if b is false.
func Dirty(b bool) []string {
	if b {
		return os.Environ()
	}

	return nil
}

// not bound by compatibility guarantees. do not use.
func UnsafeIsLocalCompute() bool {
	const niluid = "00000000-0000-0000-0000-000000000000"
	return String(niluid, eg.EnvComputeAccountID) == niluid
}
