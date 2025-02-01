package golangcov

// useful reference code.
// https://cs.opensource.google/go/go/+/refs/tags/go1.23.5:src/cmd/cover/func.go

import (
	"context"
	"fmt"
	"io/fs"
	"iter"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/internal/coverage"
	"golang.org/x/tools/cover"
)

func Coverage(ctx context.Context, dir string) iter.Seq2[*coverage.Report, error] {
	return func(yield func(*coverage.Report, error) bool) {
		err := fs.WalkDir(os.DirFS(dir), ".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				return nil
			}

			profiles, err := cover.ParseProfiles(filepath.Join(dir, path))
			if err != nil {
				return err
			}

			for _, profile := range profiles {
				ok := yield(&coverage.Report{
					Path:     profile.FileName,
					Coverage: percentCovered(profile),
				}, nil)
				if !ok {
					return fmt.Errorf("yield failed")
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}

			return nil
		})

		if err != nil {
			yield(nil, err)
		}
	}
}

// percentCovered returns, as a percentage, the fraction of the statements in
// the profile covered by the test run.
// In effect, it reports the coverage of a given source file.
func percentCovered(p *cover.Profile) float32 {
	var total, covered int64
	for _, b := range p.Blocks {
		total += int64(b.NumStmt)
		if b.Count > 0 {
			covered += int64(b.NumStmt)
		}
	}
	if total == 0 {
		return 0
	}
	return float32(float64(covered) / float64(total) * 100)
}
