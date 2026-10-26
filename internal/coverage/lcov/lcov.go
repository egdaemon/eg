package lcov

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/errorsx"
)

const (
	prefixSourceFile         = "SF:"
	prefixFunction           = "FN:"
	prefixFunctionHits       = "FNDA:"
	prefixExecutableLines    = "LF:"
	prefixExecutableLinesHit = "LH:"
	prefixBranches           = "BRF:"
	prefixBranchesHit        = "BRH:"
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
			path        string
			linesHit    HitCount
			branceshHit HitCount
			fnhits      = make(map[string]int64)
		)

		scanner := bufio.NewScanner(src)

		for scanner.Scan() {
			line := scanner.Text()

			if after, ok := strings.CutPrefix(line, prefixSourceFile); ok {
				path = strings.TrimSpace(after)
				continue
			}

			if after, ok := strings.CutPrefix(line, prefixFunctionHits); ok {
				rest := strings.TrimSpace(after)
				shits, name, ok := strings.Cut(rest, ",")
				if !ok {
					yield(nil, errorsx.Wrapf(fmt.Errorf("missing function name"), "invalid line %s", line))
					return
				}

				hits, err := strconv.ParseInt(strings.TrimSpace(shits), 10, 64)
				if err != nil {
					yield(nil, errorsx.Wrapf(err, "invalid line %s", line))
					return
				}

				fnhits[name] = hits
				continue
			}

			if after, ok := strings.CutPrefix(line, prefixFunction); ok {
				_, name, ok := strings.Cut(strings.TrimSpace(after), ",")
				if !ok {
					yield(nil, errorsx.Wrapf(fmt.Errorf("missing function name"), "invalid line %s", line))
					return
				}

				if _, ok := fnhits[name]; !ok {
					fnhits[name] = 0
				}
				continue
			}

			if after, ok := strings.CutPrefix(line, prefixExecutableLines); ok {
				if nLine, err := strconv.Atoi(strings.TrimSpace(after)); err != nil {
					yield(nil, errorsx.Wrapf(err, "invalid line %s", line))
					return
				} else {
					linesHit.Total = nLine
					continue
				}
			}

			if after, ok := strings.CutPrefix(line, prefixExecutableLinesHit); ok {
				if nLine, err := strconv.Atoi(strings.TrimSpace(after)); err != nil {
					yield(nil, errorsx.Wrapf(err, "invalid line %s", line))
					return
				} else {
					linesHit.Hit = nLine
					continue
				}
			}

			if after, ok := strings.CutPrefix(line, prefixBranches); ok {
				if nLine, err := strconv.Atoi(strings.TrimSpace(after)); err != nil {
					yield(nil, errorsx.Wrapf(err, "invalid line %s", line))
					return
				} else {
					branceshHit.Total = nLine
					continue
				}
			}

			if after, ok := strings.CutPrefix(line, prefixBranchesHit); ok {
				if nLine, err := strconv.Atoi(strings.TrimSpace(after)); err != nil {
					yield(nil, errorsx.Wrapf(err, "invalid line %s", line))
					return
				} else {
					branceshHit.Hit = nLine
					continue
				}
			}

			if strings.HasPrefix(line, prefixEnd) {
				if !yield(&coverage.Report{
					Path:       path,
					Statements: linesHit.Coverage(),
					Branches:   branceshHit.Coverage(),
				}, nil) {
					clear(fnhits)
					return
				}

				for name, hits := range fnhits {
					if !yield(&coverage.Report{
						Path:       path,
						Statements: linesHit.Coverage(),
						Branches:   branceshHit.Coverage(),
						Fnname:     name,
						Hits:       hits,
					}, nil) {
						clear(fnhits)
						return
					}
				}

				path = ""
				linesHit = HitCount{}
				branceshHit = HitCount{}
				clear(fnhits)

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
				if !yield(rep, err) {
					return nil
				}
			}

			return nil
		})

		if err != nil {
			yield(nil, err)
		}
	}
}
