package lcov

import (
	"bufio"
	"context"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/interp/events"
)

const (
	prefixSourceFile         = "SF:"
	prefixExecutableLines    = "LF:"
	prefixExecutableLinesHit = "LH:"
	prefixEnd                = "end_of_record"
)

type HitCount struct {
	Total int
	Hit   int
}

func (t HitCount) Coverage() float32 {
	if t.Total == 0 {
		return 0.0
	}

	return (float32(t.Hit) / float32(t.Total)) * 100.0
}

func Parse(ctx context.Context, src io.Reader) iter.Seq2[*coverage.Report, error] {
	return func(yield func(*coverage.Report, error) bool) {
		var (
			path     string
			linesHit HitCount
		)

		scanner := bufio.NewScanner(src)

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, prefixSourceFile) {
				path = strings.TrimSpace(strings.TrimPrefix(line, prefixSourceFile))
				continue
			}

			if strings.HasPrefix(line, prefixExecutableLines) {
				if nLine, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, prefixExecutableLines))); err != nil {
					yield(nil, errorsx.Wrapf(err, "invalid line %s", line))
					return
				} else {
					linesHit.Total = nLine
					continue
				}
			}

			if strings.HasPrefix(line, prefixExecutableLinesHit) {
				if nLine, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, prefixExecutableLinesHit))); err != nil {
					yield(nil, errorsx.Wrapf(err, "invalid line %s", line))
					return
				} else {
					linesHit.Hit = nLine
					continue
				}
			}

			if strings.HasPrefix(line, prefixEnd) {
				ok := yield(&events.Coverage{
					Path:     path,
					Coverage: linesHit.Coverage(),
				}, nil)
				if !ok {
					return
				}

				select {
				case <-ctx.Done():
					break
				default:
					continue
				}
			}
		}

		failed := errorsx.Compact(
			errorsx.Wrap(scanner.Err(), "failed to read lcov"),
			ctx.Err(),
		)
		if failed != nil {
			yield(nil, failed)
		}
	}
}

func Coverage(ctx context.Context, dir string) iter.Seq2[*coverage.Report, error] {
	return func(yield func(*coverage.Report, error) bool) {

		err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return errorsx.Wrapf(err, "failed: %s", filepath.Join(dir, path))
			}

			if d.IsDir() {
				return nil
			}

			// for now only consider lcov.info files.
			if d.Name() != "lcov.info" {
				return nil
			}

			info, err := os.Open(filepath.Join(dir, path))
			if err != nil {
				return errorsx.Wrapf(err, "unable to open lcov file: %s", path)
			}
			defer info.Close()

			for rep, err := range Parse(ctx, info) {
				yield(rep, err)
			}

			return nil
		})

		if err != nil {
			yield(nil, err)
		}
	}
}
