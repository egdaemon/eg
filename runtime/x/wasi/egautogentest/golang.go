package egautogentest

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egllm"
)

const golangPromptTemplate = `
test the following codeblock using the given coding style and example usage blocks as guidance for how to structure the code.
rules:
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
// already exercise it, into a single prompt for Model.
type Golang struct {
	Model string
	Style string
}

func (t Golang) Generate(seq iterx.Seq[Fn]) eg.OpFn {
	op := func(ctx context.Context, _ eg.Op) (err error) {
		loaded := map[string][]*packages.Package{}

		for fn := range seq.Each(ctx) {
			dir := filepath.Dir(fn.Path)

			pkgs, ok := loaded[dir]
			if !ok {
				if pkgs, err = loadPackages(dir); err != nil {
					return err
				}
				loaded[dir] = pkgs
			}

			if err := t.generate(ctx, fn, pkgs); err != nil {
				return err
			}
		}

		return seq.Err()
	}

	return egllm.With(t.Model, op)
}

func loadPackages(dir string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:   dir,
		Tests: true,
	}

	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, errorsx.Wrapf(err, "unable to load package: %s", dir)
	}

	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			return nil, fmt.Errorf("package %s failed to load: %v", p.PkgPath, p.Errors)
		}
	}

	return pkgs, nil
}

func (t Golang) generate(ctx context.Context, fn Fn, pkgs []*packages.Package) error {
	decl, fset, pkg := findFuncDecl(pkgs, fn.Name)
	if decl == nil {
		return fmt.Errorf("unable to locate function %s in %s", fn.Name, fn.Path)
	}

	codeblock, err := renderNode(fset, decl)
	if err != nil {
		return errorsx.Wrap(err, "unable to render function")
	}

	prompt := strings.NewReplacer(
		":sample:", t.Style,
		":usage:", collectUsage(pkgs, fn.Name),
		":types:", collectTypes(pkg, fset, decl),
		":codeblock:", codeblock,
		":focus:", fn.Name,
	).Replace(golangPromptTemplate)

	result, err := egllm.Generate(ctx, egllm.New(), t.Model, prompt)
	if err != nil {
		return err
	}

	dest := strings.TrimSuffix(fn.Path, filepath.Ext(fn.Path)) + "_autogentest_test.go"
	return os.WriteFile(dest, []byte(result), 0644)
}

// findFuncDecl locates the top level function declaration named name across
// the loaded package set (including its test variants).
func findFuncDecl(pkgs []*packages.Package, name string) (*ast.FuncDecl, *token.FileSet, *packages.Package) {
	for _, p := range pkgs {
		for _, f := range p.Syntax {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name {
					return fd, p.Fset, p
				}
			}
		}
	}

	return nil, nil, nil
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
func collectTypes(pkg *packages.Package, fset *token.FileSet, decl *ast.FuncDecl) string {
	if pkg == nil || pkg.TypesInfo == nil {
		return ""
	}

	seen := map[string]bool{}
	var names []string

	ast.Inspect(decl, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		obj := pkg.TypesInfo.Uses[ident]
		if obj == nil {
			obj = pkg.TypesInfo.Defs[ident]
		}
		if obj == nil {
			return true
		}

		named, ok := obj.Type().(*types.Named)
		if !ok {
			return true
		}

		tobj := named.Obj()
		if tobj.Pkg() == nil || tobj.Pkg() != pkg.Types {
			return true
		}

		if seen[tobj.Name()] {
			return true
		}
		seen[tobj.Name()] = true
		names = append(names, tobj.Name())

		return true
	})

	sort.Strings(names)

	rendered := make([]string, 0, len(names))
	for _, name := range names {
		spec := findTypeSpec(pkg, name)
		if spec == nil {
			continue
		}

		// render as its own "type X ..." declaration regardless of whether
		// it was originally declared inside a grouped type (...) block.
		decl := &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{spec}}
		if s, err := renderNode(fset, decl); err == nil {
			rendered = append(rendered, s)
		}
	}

	return strings.Join(rendered, "\n\n")
}

func findTypeSpec(pkg *packages.Package, name string) *ast.TypeSpec {
	for _, f := range pkg.Syntax {
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

// collectUsage finds existing Test/Example/Fuzz functions across pkgs that
// already call a function named name, rendering up to maxUsageExamples of
// them whole as real usage examples.
func collectUsage(pkgs []*packages.Package, name string) string {
	var examples []string

	for _, p := range pkgs {
		for _, f := range p.Syntax {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Recv != nil || !isTestFuncName(fd.Name.Name) || !callsFunction(fd, name) {
					continue
				}

				if rendered, err := renderNode(p.Fset, fd); err == nil {
					examples = append(examples, rendered)
				}

				if len(examples) >= maxUsageExamples {
					break
				}
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
