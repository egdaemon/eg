package egautogentest

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/egdaemon/eg/internal/coverage/golangcov"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
)

const golangPromptTemplate = `
test the following codeblock using the given coding style and example usage blocks as guidance for how to structure the code.
rules:
- you must respond with a single complete go test file, including its package clause and imports, in a single go code block.
- you must not include *any* comments
- you must not share variables between tests.
- you must not use or create mocking/stub or any kind of code that acts as a substitute that you do not find in samples.
- you must not do not write commented out code.
- you must not do not document the tests.
- you must omit the test and print all the code you skipped at the end if you can't write an effective test.
- you must ensure the test cases are comprehensive.

--------------------------------------------------- STYLE EXAMPLES ---------------------------------------------------
:sample:
--------------------------------------------------- USAGE EXAMPLES ---------------------------------------------------
:usage:
---------------------------------------------------     TYPES      ---------------------------------------------------
:types:
---------------------------------------------------   CODE BLOCK   ---------------------------------------------------
:codeblock:
---------------------------------------------------     FOCUS      ---------------------------------------------------
:focus:
`

// Golang generates tests for Go functions, combining the target function's
// source, the named types it operates on, and any existing tests that
// already exercise it, into a single prompt for Model. generated tests are
// only kept if they compile and pass.
type Golang struct {
	Model    string
	Style    string
	Attempts int // number of generations to try per function, defaults to 1.
}

func (t Golang) Generate(seq iterx.Seq[Fn]) eg.OpFn {
	op := func(ctx context.Context, _ eg.Op) (err error) {
		loaded := map[string]*gopkg{}

		for fn := range seq.Each(ctx) {
			dir := filepath.Dir(fn.Path)

			pkg, ok := loaded[dir]
			if !ok {
				if pkg, err = loadPackage(dir); err != nil {
					return err
				}
				loaded[dir] = pkg
			}

			if err := t.generate(ctx, fn, pkg); err != nil {
				return err
			}
		}

		return seq.Err()
	}

	return egllm.With(t.Model, op)
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

func (t Golang) generate(ctx context.Context, fn Fn, pkg *gopkg) error {
	decl := findFuncDecl(pkg, fn.Name)
	if decl == nil {
		return fmt.Errorf("unable to locate function %s in %s", fn.Name, fn.Path)
	}

	codeblock, err := renderNode(pkg.fset, decl)
	if err != nil {
		return errorsx.Wrap(err, "unable to render function")
	}

	prompt := strings.NewReplacer(
		":sample:", t.Style,
		":usage:", collectUsage(pkg, decl.Name.Name),
		":types:", collectTypes(pkg, decl),
		":codeblock:", codeblock,
		":focus:", fn.Name,
	).Replace(golangPromptTemplate)

	dir := filepath.Dir(fn.Path)
	dest := strings.TrimSuffix(fn.Path, filepath.Ext(fn.Path)) + "_autogentest_test.go"

	for attempt := 1; attempt <= max(t.Attempts, 1); attempt++ {
		result, err := egllm.Generate(ctx, egllm.New(), t.Model, prompt)
		if err != nil {
			return err
		}

		src, tests, err := extractTest(result, pkg.name)
		if err != nil {
			log.Printf("autogentest %s attempt %d: discarding response: %v\n", fn.Name, attempt, err)
			continue
		}

		if err := os.WriteFile(dest, src, 0644); err != nil {
			return err
		}

		cmd := eggolang.Runtime().Directory(dir).Newf("go test -count=1 -run '^(%s)$' .", strings.Join(tests, "|"))
		if err := shell.Run(ctx, cmd); err != nil {
			log.Printf("autogentest %s attempt %d: discarding failing test: %v\n", fn.Name, attempt, err)
			if err := os.Remove(dest); err != nil {
				return err
			}
			continue
		}

		return nil
	}

	log.Printf("autogentest %s: unable to generate a passing test\n", fn.Name)
	return nil
}

var fencedcode = regexp.MustCompile("(?s)```[a-zA-Z]*[ \t]*\r?\n(.*?)```")

// extractTest pulls the test file out of a model response, forces it into the
// given package, and formats it. it returns the names of the tests within.
func extractTest(response string, pkgname string) ([]byte, []string, error) {
	code := response
	for _, m := range fencedcode.FindAllStringSubmatch(response, -1) {
		if strings.Contains(m[1], "func Test") {
			code = m[1]
			break
		}
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", code, parser.ParseComments|parser.SkipObjectResolution)
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
