package golangcov

// useful reference code.
// https://cs.opensource.google/go/go/+/refs/tags/go1.23.5:src/cmd/cover/func.go

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/astcodec"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
	"golang.org/x/mod/modfile"
	"golang.org/x/tools/cover"
)

// Resolver maps a profile's file name (an import path such as
// example.com/mod/pkg/file.go) onto the filesystem. unresolved names are
// returned unchanged with ok false.
type Resolver func(name string) (path string, ok bool)

// Files resolves nothing, restricting Coverage to per file reports.
func Files(name string) (path string, ok bool) {
	return name, false
}

// Modules resolves profile file names against the given go.mod files.
func Modules(gomods iter.Seq[string]) Resolver {
	roots := map[string]string{}
	for gomod := range gomods {
		raw, err := os.ReadFile(gomod)
		if err != nil {
			continue
		}

		if mpath := modfile.ModulePath(raw); mpath != "" {
			roots[mpath] = filepath.Dir(gomod)
		}
	}

	return func(name string) (string, bool) {
		// longest matching module path wins, nested modules are more specific.
		match := ""
		for mpath := range roots {
			if len(mpath) > len(match) && strings.HasPrefix(name, mpath+"/") {
				match = mpath
			}
		}

		if match == "" {
			return name, false
		}

		return filepath.Join(roots[match], strings.TrimPrefix(name, match+"/")), true
	}
}

// Coverage reports per file and per function coverage for the profiles in dir.
// per function reports require the source, so they're only generated for files
// resolve can locate. use Files for per file coverage only.
func Coverage(ctx context.Context, dir string, resolve Resolver) iter.Seq2[*coverage.Report, error] {
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
				return errorsx.Wrapf(err, "unable to parse profiles: %s", path)
			}

			for _, profile := range profiles {
				path, resolved := resolve(profile.FileName)

				ok := yield(&coverage.Report{
					Path:       path,
					Statements: percentCovered(profile.Blocks),
				}, nil)
				if !ok {
					return iterx.ErrIterationComplete
				}

				if resolved {
					for _, fn := range functions(path, profile) {
						if !yield(fn, nil) {
							return iterx.ErrIterationComplete
						}
					}
				}

				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
			}

			return nil
		})

		// the consumer stopped iterating, yield must not be called again.
		if errorsx.Ignore(err, iterx.ErrIterationComplete) != nil {
			yield(nil, err)
		}
	}
}

// functions reports coverage for each function declared in the source file at path.
// a function's hits is the count of its first block, i.e. the number of times it was entered.
// unparsable source yields no reports.
func functions(path string, p *cover.Profile) (reports []*coverage.Report) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil
	}

	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}

		start, end := fset.Position(fd.Pos()), fset.Position(fd.End())
		var blocks []cover.ProfileBlock
		for _, b := range p.Blocks {
			if before(b.StartLine, b.StartCol, start.Line, start.Column) || before(end.Line, end.Column, b.EndLine, b.EndCol) {
				continue
			}
			blocks = append(blocks, b)
		}

		if len(blocks) == 0 {
			continue
		}

		// profile blocks are sorted by position, so the first is the function's entry.
		reports = append(reports, &coverage.Report{
			Path:       path,
			Fnname:     astcodec.FuncName(fd),
			Hits:       int64(blocks[0].Count),
			Statements: percentCovered(blocks),
		})
	}

	return reports
}

func before(line, col, oline, ocol int) bool {
	return line < oline || (line == oline && col < ocol)
}

// percentCovered returns, as a percentage, the fraction of the statements in
// the blocks covered by the test run.
func percentCovered(blocks []cover.ProfileBlock) float32 {
	var total, covered int64
	for _, b := range blocks {
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
