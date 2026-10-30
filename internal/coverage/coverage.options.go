package coverage

import (
	"context"
	"errors"
	"iter"
	"path/filepath"
	"slices"
	"strings"
)

// Seq is a sequence of coverage reports, e.g.) golangcov.Coverage or lcov.Parse.
type Seq = iter.Seq2[*Report, error]

// Source produces the reports when the options are resolved, it is not called before.
type Source func(context.Context) Seq

// Transform rewrites, filters, or annotates a Seq.
type Transform func(Seq) Seq

type option struct {
	src        Source
	root       string
	transforms []Transform
}

type Option func(*option)

// Options builds a Source with transforms applied to it.
type Options []Option

// New starts from src.
func New(src Source) Options {
	return Options{func(o *option) { o.src = src }}
}

// Transforms applies o to the source, in order, after any previously added transforms.
func (t Options) Transforms(o ...Transform) Options {
	return append(slices.Clip(t), func(opt *option) {
		opt.transforms = append(opt.transforms, o...)
	})
}

// Root joins the relative paths of the reports onto dir, before any transform is applied. absolute paths are left
// as is. dir is expected to carry the workspace prefix already, e.g.) egenv.WorkingDirectory("console").
func (t Options) Root(dir string) Options {
	return append(slices.Clip(t), func(opt *option) {
		opt.root = dir
	})
}

// MarkGeneratedExt marks the reports of files ending in any of exts as generated, e.g.) ".gen.go", ".pb.go".
func (t Options) MarkGeneratedExt(exts ...string) Options {
	return t.Transforms(TransformMarkGenerated(func(r Report) bool {
		return slices.ContainsFunc(exts, func(ext string) bool { return strings.HasSuffix(r.Path, ext) })
	}))
}

// Seq resolves the options. the first transform wraps the source, each following transform wraps the previous.
func (t Options) Seq(ctx context.Context) Seq {
	var o option
	for _, opt := range t {
		opt(&o)
	}

	if o.src == nil {
		return func(yield func(*Report, error) bool) {
			yield(nil, errors.New("no coverage source"))
		}
	}

	s := o.src(ctx)
	if o.root != "" {
		s = TransformPath(func(p string) string {
			if filepath.IsAbs(p) {
				return p
			}

			return filepath.Join(o.root, p)
		})(s)
	}
	for _, tx := range o.transforms {
		s = tx(s)
	}

	return s
}

// TransformMarkGenerated marks the reports for which any of the checks returns true as generated. reports are copied,
// never modified, and no report is dropped.
func TransformMarkGenerated(checks ...func(Report) bool) Transform {
	return func(in Seq) Seq {
		return func(yield func(*Report, error) bool) {
			for r, err := range in {
				if r != nil && slices.ContainsFunc(checks, func(check func(Report) bool) bool { return check(*r) }) {
					c := *r
					c.Generated = true
					r = &c
				}

				if !yield(r, err) {
					return
				}
			}
		}
	}
}

// TransformPath rewrites the path of each report with fn. reports are copied, never modified.
func TransformPath(fn func(string) string) Transform {
	return func(in Seq) Seq {
		return func(yield func(*Report, error) bool) {
			for r, err := range in {
				if r != nil {
					c := *r
					c.Path = fn(c.Path)
					r = &c
				}

				if !yield(r, err) {
					return
				}
			}
		}
	}
}
