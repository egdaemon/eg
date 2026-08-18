package gdx

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"strconv"
	"time"
)

func genDst() (path string, dst io.WriteCloser) {
	t := time.Now()
	ts := reverse(strconv.Itoa(int(t.Unix())))
	path = filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s-%s.trace", filepath.Base(os.Args[0]), t.Format("2006-01-02"), ts))

	f, err := os.Create(path)
	if err != nil {
		log.Println("failed to open file:", path, err)
		log.Println("routine dump falling back to stderr")
		return "stderr", nopCloser{os.Stderr}
	}

	return path, f
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

// DumpRoutinesInto writes the current goroutine stack traces into dst, then closes it.
func DumpRoutinesInto(dst io.WriteCloser) error {
	werr := pprof.Lookup("goroutine").WriteTo(dst, 1)
	cerr := dst.Close()
	if werr != nil {
		return werr
	}
	return cerr
}

// DumpRoutines writes current goroutine stack traces to a temp file
// and returns that file's path. if for some reason a file could not be opened
// it falls back to stderr.
func DumpRoutines() (path string, err error) {
	path, dst := genDst()
	return path, DumpRoutinesInto(dst)
}

// DumpOnSignal runs DumpRoutines and prints the resulting path to stderr whenever
// one of the provided signals is received.
func DumpOnSignal(ctx context.Context, sigs ...os.Signal) {
	OnSignal(ctx, func(ctx context.Context) error {
		path, err := DumpRoutines()
		if err != nil {
			return fmt.Errorf("goroutine dump failed: %w", err)
		}
		log.Println("dump located at:", path)
		return nil
	}, sigs...)
}

// OnSignal runs do whenever one of the provided signals is received, until ctx is done.
func OnSignal(ctx context.Context, do func(ctx context.Context) error, sigs ...os.Signal) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, sigs...)
	defer signal.Stop(signals)

	for {
		select {
		case <-ctx.Done():
			return
		case s := <-signals:
			log.Println("signal processing initiated", s)
			if err := do(ctx); err != nil {
				log.Println("signal processing failed", s, err)
			}
			log.Println("signal processing completed", s)
		}
	}
}
