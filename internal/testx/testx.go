package testx

import (
	"bytes"
	"context"
	"crypto/md5"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/gofrs/uuid/v5"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/require"
)

func TempDir() string {
	return ginkgo.GinkgoT().TempDir()
}

func Fixture(path ...string) string {
	return filepath.Join(".fixtures", filepath.Join(path...))
}

// Read a file at the given path.
func Read(path ...string) io.Reader {
	return bytes.NewReader(errorsx.Must(os.ReadFile(filepath.Join(path...))))
}

// ReadString from the given file.
func ReadString(path ...string) string {
	return string(errorsx.Must(os.ReadFile(filepath.Join(path...))))
}

func ReadMD5(path ...string) string {
	d := md5.New()
	_ = errorsx.Must(d.Write(errorsx.Must(os.ReadFile(filepath.Join(path...)))))
	return uuid.FromBytesOrNil(d.Sum(nil)).String()
}

// Must is a small language extension for panicing on the common
// value, error return pattern. only used in tests.
func Must[T any](v T, err error) T {
	gomega.Expect(err).To(gomega.BeNil())
	return v
}

// Must is a small language extension for panicing on the common
// value, error return pattern. only used in tests.
func MustT[T any](v T, err error) func(t testing.TB) T {
	return func(t testing.TB) T {
		require.NoError(t, err)
		return v
	}
}

// Tempenvvar temporarily set the environment variable.
func Tempenvvar(k, v string, do func()) {
	o := os.Getenv(k)
	defer os.Setenv(k, o)
	if err := os.Setenv(k, v); err != nil {
		panic(err)
	}
	do()
}

func IOMD5(in io.Reader) string {
	digester := md5.New()
	errorsx.Must(io.Copy(digester, in))
	return uuid.FromBytesOrNil(digester.Sum(nil)).String()
}

func IOString(in io.Reader) string {
	return string(errorsx.Must(io.ReadAll(in)))
}

func IOBytes(in io.Reader) []byte {
	return errorsx.Must(io.ReadAll(in))
}

func Context(t *testing.T) (context.Context, context.CancelFunc) {
	if deadline, ok := t.Deadline(); ok {
		return context.WithDeadline(context.Background(), deadline)
	}

	return context.WithCancel(context.Background())
}
