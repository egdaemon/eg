package egautogentest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/egdaemon/eg/internal/coverage/golangcov"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

var golangPromptTemplate = template.Must(template.New("golang").Parse(`
test the following codeblock using the given coding style and example usage blocks as guidance for how to structure the code.
rules:
{{range .Ensure}}- {{.}}
{{end}}
--------------------------------------------------- STYLE EXAMPLES ---------------------------------------------------
{{.Style}}
--------------------------------------------------- USAGE EXAMPLES ---------------------------------------------------
{{.Usage}}
---------------------------------------------------     TYPES      ---------------------------------------------------
{{.Types}}
{{if .Existing}}---------------------------------------------------  EXISTING TESTS ---------------------------------------------------
this test file already exists, your response replaces it so it must contain all of its existing tests along with the new ones.
{{.Existing}}
{{end}}---------------------------------------------------   CODE BLOCK   ---------------------------------------------------
{{.Codeblock}}
---------------------------------------------------     FOCUS      ---------------------------------------------------
{{.Fn.Name}}
`))

// golangDefaultRules are the rules used when none are given.
var golangDefaultRules = []string{
	"you must respond with only a single complete go test file, including its package clause and imports, as plain text without code fences or any other text.",
	"you must not include *any* comments",
	"you must not share variables between tests.",
	"you must not use or create mocking/stub or any kind of code that acts as a substitute that you do not find in samples.",
	"you must not do not write commented out code.",
	"you must not do not document the tests.",
	"you must omit the test if you can't write an effective test.",
	"you must ensure the test cases are comprehensive.",
}

// golangPrompt is the data the golang prompt template is executed with.
type golangPrompt struct {
	PromptContext
	Style     string
	Usage     string
	Types     string
	Codeblock string
	Existing  string // contents of the test file being generated, if it already exists.
}

// golangOption is the resolved configuration of the Golang generator.
type golangOption struct {
	seq      iterx.Seq[Fn]
	style    string
	filename string
	pkgs     *pkgcache
}

// DefaultGolangFilename is the pattern for the name of the file generated tests are written to.
const DefaultGolangFilename = "%fn.file%.%fn.name%.autogentest_test.go"

// GolangOption configures the Golang generator.
type GolangOption func(*golangOption)

// GolangOptions is a chainable set of GolangOption, it implements Generator.
type GolangOptions []GolangOption

// Golang generates tests for the Go functions in seq, combining the target function's
// source, the named types it operates on, and any existing tests that
// already exercise it, into a single prompt for the model. generated tests are
// only kept if they compile and pass.
func Golang(seq iterx.Seq[Fn]) GolangOptions {
	pkgs := &pkgcache{loaded: map[string]*gopkg{}}
	return GolangOptions{func(o *golangOption) {
		o.seq = seq
		o.pkgs = pkgs
	}}
}

// Style sets an example test the generated tests should follow.
func (t GolangOptions) Style(v string) GolangOptions {
	return append(t, func(o *golangOption) {
		o.style = v
	})
}

// Filename sets the pattern for the name of the file generated tests are written to,
// next to the function's source. the file name must end in _test.go. in addition to the
// git patterns supported by eggit.StringReplace, the following are replaced:
//
//	%fn.file% -> the name of the file declaring the function, without its extension.
//	%fn.name% -> the lower cased name of the function, methods are Type_method.
//
// defaults to DefaultGolangFilename.
func (t GolangOptions) Filename(pattern string) GolangOptions {
	return append(t, func(o *golangOption) {
		o.filename = pattern
	})
}

// golangFilename resolves the file name pattern for fn.
func golangFilename(pattern string, fn Fn) (string, error) {
	name := strings.NewReplacer(
		"%fn.file%", strings.TrimSuffix(filepath.Base(fn.Path), filepath.Ext(fn.Path)),
		"%fn.name%", strings.ToLower(strings.ReplaceAll(fn.Name, ".", "_")),
	).Replace(pattern)
	name = eggit.StringReplace(name)

	if name != filepath.Base(name) {
		return "", fmt.Errorf("file name must not contain a path: %s", name)
	}

	if !strings.HasSuffix(name, "_test.go") {
		return "", fmt.Errorf("file name must end in _test.go: %s", name)
	}

	return name, nil
}

func (t GolangOptions) resolve() (o golangOption) {
	o.filename = DefaultGolangFilename
	for _, opt := range t {
		opt(&o)
	}

	return o
}

func (t GolangOptions) Seq() iterx.Seq[Fn] {
	return t.resolve().seq
}

func (t GolangOptions) Prompt(c PromptContext) (string, error) {
	o := t.resolve()

	pkg, err := o.pkgs.load(filepath.Dir(c.Fn.Path))
	if err != nil {
		return "", err
	}

	decl := findFuncDecl(pkg, c.Fn.Name)
	if decl == nil {
		return "", fmt.Errorf("unable to locate function %s in %s", c.Fn.Name, c.Fn.Path)
	}

	codeblock, err := renderNode(pkg.fset, decl)
	if err != nil {
		return "", errorsx.Wrap(err, "unable to render function")
	}

	if len(c.Ensure) == 0 {
		c.Ensure = golangDefaultRules
	}

	name, err := golangFilename(o.filename, c.Fn)
	if err != nil {
		return "", err
	}

	var existing string
	if prev, err := os.ReadFile(filepath.Join(filepath.Dir(c.Fn.Path), name)); err == nil {
		existing = string(prev)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", errorsx.Wrap(err, "unable to read existing tests")
	}

	var buf strings.Builder
	if err := golangPromptTemplate.Execute(&buf, golangPrompt{
		PromptContext: c,
		Style:         o.style,
		Usage:         collectUsage(pkg, decl.Name.Name),
		Types:         collectTypes(pkg, decl),
		Codeblock:     codeblock,
		Existing:      existing,
	}); err != nil {
		return "", errorsx.Wrap(err, "unable to render prompt")
	}

	return buf.String(), nil
}

// Verify keeps the generated test next to the function only if it compiles and passes.
// the test is run in place of any existing test file by overlaying it, so the existing file is only
// replaced once the generated test passes.
func (t GolangOptions) Verify(ctx context.Context, fn Fn, artifact string) error {
	o := t.resolve()
	dir := filepath.Dir(fn.Path)

	pkg, err := o.pkgs.load(dir)
	if err != nil {
		return err
	}

	src, tests, err := extractTest(artifact, pkg.name)
	if err != nil {
		return err
	}

	name, err := golangFilename(o.filename, fn)
	if err != nil {
		return err
	}

	dest := filepath.Join(dir, name)

	tmp, err := writeTemp(dir, ".autogentest.*", src)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)

	overlay, err := writeOverlay(dir, dest, tmp)
	if err != nil {
		return err
	}
	defer os.Remove(overlay)

	cmd := eggolang.Runtime().Directory(dir).Newf("go test -count=1 -overlay '%s' -run '^(%s)$' .", overlay, strings.Join(tests, "|"))
	log.Printf("autogentest %s: verifying dest=%s tmp=%s overlay=%s tests=%v\n", fn.Name, dest, tmp, overlay, tests)
	if err := shell.Run(ctx, cmd.Debug()); err != nil {
		return err
	}

	return os.Rename(tmp, dest)
}

// writeOverlay writes a go build overlay presenting the file at src as the one at dest.
func writeOverlay(dir string, dest string, src string) (string, error) {
	absdest, err := filepath.Abs(dest)
	if err != nil {
		return "", err
	}

	abssrc, err := filepath.Abs(src)
	if err != nil {
		return "", err
	}

	encoded, err := json.Marshal(struct {
		Replace map[string]string
	}{Replace: map[string]string{absdest: abssrc}})
	if err != nil {
		return "", err
	}

	return writeTemp(dir, ".autogentest.overlay.*.json", encoded)
}

// pkgcache loads each directory's package once.
type pkgcache struct {
	loaded map[string]*gopkg
}

func (t *pkgcache) load(dir string) (pkg *gopkg, err error) {
	if pkg, ok := t.loaded[dir]; ok {
		return pkg, nil
	}

	if pkg, err = loadPackage(dir); err != nil {
		return nil, err
	}

	t.loaded[dir] = pkg
	return pkg, nil
}

// gopkg is the syntax of a single directory's go files, test files included.
// it's built with go/parser alone because go/packages shells out to the go
// command, which isn't possible from within the wasi runtime.
type gopkg struct {
	name  string // package name of the non test files.
	fset  *token.FileSet
	files []*ast.File
}

func loadPackage(dir string) (*gopkg, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errorsx.Wrapf(err, "unable to read package: %s", dir)
	}

	p := &gopkg{fset: token.NewFileSet()}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".go" {
			continue
		}

		f, err := parser.ParseFile(p.fset, filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, errorsx.Wrapf(err, "unable to parse: %s", filepath.Join(dir, e.Name()))
		}

		if p.name == "" && !strings.HasSuffix(e.Name(), "_test.go") {
			p.name = f.Name.Name
		}

		p.files = append(p.files, f)
	}

	if p.name == "" {
		return nil, fmt.Errorf("no go source files found: %s", dir)
	}

	return p, nil
}

// extractTest pulls the test file out of a model response, forces it into the
// given package, and formats it. it returns the names of the tests within.
func extractTest(response string, pkgname string) ([]byte, []string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", response, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, errorsx.Wrap(err, "invalid go source")
	}

	f.Name.Name = pkgname

	var tests []string
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && strings.HasPrefix(fd.Name.Name, "Test") {
			tests = append(tests, fd.Name.Name)
		}
	}

	if len(tests) == 0 {
		return nil, nil, fmt.Errorf("no tests found")
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, nil, errorsx.Wrap(err, "unable to format")
	}

	return buf.Bytes(), tests, nil
}

// findFuncDecl locates the top level function declaration named name across
// the package's files. methods are named Type.Method.
func findFuncDecl(pkg *gopkg, name string) *ast.FuncDecl {
	for _, f := range pkg.files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && golangcov.FuncName(fd) == name {
				return fd
			}
		}
	}

	return nil
}

func renderNode(fset *token.FileSet, node ast.Node) (string, error) {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, node); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// collectTypes walks decl looking for references to named types declared
// within pkg, and renders each of their declarations so the model can see
// the shapes the function actually operates on.
func collectTypes(pkg *gopkg, decl *ast.FuncDecl) string {
	seen := map[string]bool{}

	ast.Inspect(decl, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok {
			seen[ident.Name] = true
		}

		return true
	})

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)

	rendered := make([]string, 0, len(names))
	for _, name := range names {
		spec := findTypeSpec(pkg, name)
		if spec == nil {
			continue
		}

		// render as its own "type X ..." declaration regardless of whether
		// it was originally declared inside a grouped type (...) block.
		gd := &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
		if s, err := renderNode(pkg.fset, gd); err == nil {
			rendered = append(rendered, s)
		}
	}

	return strings.Join(rendered, "\n\n")
}

// findTypeSpec locates a type declared by the package's non test files.
func findTypeSpec(pkg *gopkg, name string) *ast.TypeSpec {
	for _, f := range pkg.files {
		if strings.HasSuffix(pkg.fset.Position(f.Package).Filename, "_test.go") {
			continue
		}

		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, spec := range gd.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == name {
					return ts
				}
			}
		}
	}

	return nil
}

const maxUsageExamples = 3

// collectUsage finds existing Test/Example/Fuzz functions across pkg that
// already call a function named name, rendering up to maxUsageExamples of
// them whole as real usage examples.
func collectUsage(pkg *gopkg, name string) string {
	var examples []string

	for _, f := range pkg.files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || !isTestFuncName(fd.Name.Name) || !callsFunction(fd, name) {
				continue
			}

			if rendered, err := renderNode(pkg.fset, fd); err == nil {
				examples = append(examples, rendered)
			}

			if len(examples) >= maxUsageExamples {
				return strings.Join(examples, "\n\n")
			}
		}
	}

	if len(examples) == 0 {
		return "no existing usage of this function was found."
	}

	return strings.Join(examples, "\n\n")
}

func isTestFuncName(name string) bool {
	return strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Example") || strings.HasPrefix(name, "Fuzz")
}

func callsFunction(fd *ast.FuncDecl, name string) bool {
	found := false

	ast.Inspect(fd, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		switch fn := call.Fun.(type) {
		case *ast.Ident:
			found = found || fn.Name == name
		case *ast.SelectorExpr:
			found = found || fn.Sel.Name == name
		}

		return true
	})

	return found
}
