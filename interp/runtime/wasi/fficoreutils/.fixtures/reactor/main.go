// reactor used by the fficoreutils tests in place of the coreutils reactor: it exports the same interface
// (uu_alloc, uu_free, uu_chdir and one uu_<name> per utility) for the utilities below. built with
// -buildmode=c-shared, so _initialize runs the Go runtime and main never does.
package main

import (
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

// allocations handed to the host, kept alive until it releases them.
var allocations = map[int32][]byte{}

func main() {}

//go:wasmexport uu_alloc
func alloc(n int32) int32 {
	var (
		b   = make([]byte, max(n, 1))
		ptr = int32(uintptr(unsafe.Pointer(unsafe.SliceData(b))))
	)

	allocations[ptr] = b
	return ptr
}

//go:wasmexport uu_free
func free(ptr int32, _ int32) {
	delete(allocations, ptr)
}

//go:wasmexport uu_chdir
func chdir(ptr int32, n int32) int32 {
	if os.Chdir(string(allocations[ptr][:n])) != nil {
		return 44 // ENOENT
	}
	return 0
}

// argv decodes the NUL terminated entries the host wrote, argv[0] included.
func argv(ptr int32, n int32) []string {
	return strings.Split(strings.TrimSuffix(string(allocations[ptr][:n]), "\x00"), "\x00")
}

// cat copies its files, or stdin without any, to stdout. it returns 1 when a file can't be read.
//
//go:wasmexport uu_cat
func cat(ptr int32, n int32) int32 {
	var (
		b    []byte
		err  error
		args = argv(ptr, n)[1:]
	)

	if len(args) == 0 {
		io.Copy(os.Stdout, os.Stdin)
		return 0
	}

	for _, path := range args {
		if b, err = os.ReadFile(path); err != nil {
			os.Stderr.WriteString("cat: " + err.Error() + "\n")
			return 1
		}
		os.Stdout.Write(b)
	}

	return 0
}

// yes writes y lines until the write fails, which only a closed stdout does.
//
//go:wasmexport uu_yes
func yes(ptr int32, n int32) int32 {
	for {
		if _, err := os.Stdout.Write([]byte("y\n")); err != nil {
			return 1
		}
	}
}

// sleep sleeps for its duration, e.g. 200ms.
//
//go:wasmexport uu_sleep
func sleep(ptr int32, n int32) int32 {
	var (
		d   time.Duration
		err error
	)

	if d, err = time.ParseDuration(argv(ptr, n)[1]); err != nil {
		return 1
	}
	time.Sleep(d)
	return 0
}

// where reports the directory relative paths resolve against.
//
//go:wasmexport uu_where
func where(ptr int32, n int32) int32 {
	var (
		dir string
		err error
	)

	if dir, err = os.Getwd(); err != nil {
		return 1
	}
	os.Stdout.WriteString(dir + "\n")
	return 0
}

// echo writes its arguments to stdout.
//
//go:wasmexport uu_echo
func echo(ptr int32, n int32) int32 {
	os.Stdout.WriteString(strings.Join(argv(ptr, n)[1:], " ") + "\n")
	return 0
}

// upper copies stdin to stdout in upper case.
//
//go:wasmexport uu_upper
func upper(ptr int32, n int32) int32 {
	var (
		b   []byte
		err error
	)

	if b, err = io.ReadAll(os.Stdin); err != nil {
		return 1
	}
	os.Stdout.Write(bytes.ToUpper(b))
	return 0
}

// code exits with its argument.
//
//go:wasmexport uu_code
func code(ptr int32, n int32) int32 {
	var (
		c   int
		err error
	)

	if c, err = strconv.Atoi(argv(ptr, n)[1]); err != nil {
		return 2
	}
	return int32(c)
}

// trap fails the way a panic does in the coreutils reactor, which leaves the instance unusable.
//
//go:wasmexport uu_trap
func trap(ptr int32, n int32) int32 {
	var b *bytes.Buffer
	return int32(b.Len())
}
