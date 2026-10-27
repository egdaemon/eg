package astcodec_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg/astbuild"
	"github.com/egdaemon/eg/astcodec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/packages"
)

const example = `package example

import (
	"context"
	"fmt"
	eg "github.com/egdaemon/eg/runtime/wasi/eg"
)

func main() {
	fmt.Println("main")
}

func a(ctx context.Context, op eg.Op) error {
	return nil
}

func b(ctx context.Context, op eg.Op) error {
	return nil
}

func unnamed(context.Context, eg.Op) (int, error) {
	return 0, nil
}

func noresults() {}

var v = 1
`

func parse(t *testing.T, src string) (*token.FileSet, *ast.File, *packages.Package) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "example.go", src, parser.ParseComments)
	require.NoError(t, err)
	return fset, f, &packages.Package{Syntax: []*ast.File{f}, Fset: fset}
}

func render(t *testing.T, fset *token.FileSet, n ast.Node) string {
	buf := bytes.NewBuffer(nil)
	require.NoError(t, printer.Fprint(buf, fset, n))
	return buf.String()
}

func funcnames(f *ast.File) (names []string) {
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok {
			names = append(names, fn.Name.Name)
		}
	}
	return names
}

func decl(t *testing.T, f *ast.File, name string) *ast.FuncDecl {
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn
		}
	}
	require.FailNow(t, "function not found", name)
	return nil
}

func TestFindFunctions(t *testing.T) {
	_, f, _ := parse(t, example)
	var found int
	for _, d := range f.Decls {
		if astcodec.FindFunctions(d) {
			found++
		}
	}
	assert.Equal(t, 5, found, "genDecls (imports, vars) are not functions")
}

func TestFindFunctionsByName(t *testing.T) {
	_, f, _ := parse(t, example)
	match := astcodec.FindFunctionsByName("a")

	var found []string
	for _, d := range f.Decls {
		if match(d) {
			found = append(found, d.(*ast.FuncDecl).Name.Name)
		}
	}
	assert.Equal(t, []string{"a"}, found)
}

func TestTypePattern(t *testing.T) {
	ident := ast.NewIdent
	for _, tc := range []struct {
		name     string
		pattern  []ast.Expr
		testcase []ast.Expr
		expected bool
	}{
		{name: "empty matches empty", expected: true},
		{name: "equal", pattern: []ast.Expr{ident("int"), ident("error")}, testcase: []ast.Expr{ident("int"), ident("error")}, expected: true},
		{name: "different type", pattern: []ast.Expr{ident("int")}, testcase: []ast.Expr{ident("string")}},
		{name: "order matters", pattern: []ast.Expr{ident("int"), ident("error")}, testcase: []ast.Expr{ident("error"), ident("int")}},
		{name: "shorter testcase", pattern: []ast.Expr{ident("int"), ident("error")}, testcase: []ast.Expr{ident("int")}},
		{name: "longer testcase", pattern: []ast.Expr{ident("int")}, testcase: []ast.Expr{ident("int"), ident("error")}},
		{name: "selectors", pattern: []ast.Expr{astbuild.SelExpr("context", "Context")}, testcase: []ast.Expr{astbuild.SelExpr("context", "Context")}, expected: true},
		{name: "selectors differ", pattern: []ast.Expr{astbuild.SelExpr("context", "Context")}, testcase: []ast.Expr{astbuild.SelExpr("ctx", "Context")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, astcodec.TypePattern(tc.pattern...)(tc.testcase...))
		})
	}
}

func TestMapFieldsToTypeExpr(t *testing.T) {
	_, f, _ := parse(t, "package p\nfunc f(a, b int, c string, d float64) {}\n")
	exprs := astcodec.MapFieldsToTypeExpr(decl(t, f, "f").Type.Params.List...)

	names := make([]string, 0, len(exprs))
	for _, e := range exprs {
		names = append(names, astcodec.Ident(e))
	}
	assert.Equal(t, []string{"int", "int", "string", "float64"}, names, "each name gets its own type")

	t.Run("does not mutate the provided fields", func(t *testing.T) {
		fset, f, _ := parse(t, "package p\nfunc f(int, string) (int, error) { return 0, nil }\n")
		fn := decl(t, f, "f")
		before := render(t, fset, fn)

		astcodec.MapFieldsToTypeExpr(fn.Type.Params.List...)
		astcodec.MapFieldsToTypeExpr(fn.Type.Results.List...)

		assert.Equal(t, before, render(t, fset, fn), "inspecting a function must not rename its parameters")
	})
}

func TestFieldListPattern(t *testing.T) {
	_, f, _ := parse(t, "package p\nfunc f(a, b int) (string, error) { return \"\", nil }\n")
	fn := decl(t, f, "f")

	params := astcodec.FieldListPattern(fn.Type.Params)
	require.Len(t, params, 2)

	results := astcodec.FieldListPattern(fn.Type.Results)
	require.Len(t, results, 2)
	assert.Equal(t, "string", astcodec.Ident(results[0]))
	assert.Equal(t, "error", astcodec.Ident(results[1]))

	t.Run("nil list", func(t *testing.T) {
		// functions without results have a nil field list.
		assert.NotPanics(t, func() {
			assert.Empty(t, astcodec.FieldListPattern(nil))
		})
	})
}

func TestFindFunctionsByPattern(t *testing.T) {
	_, f, _ := parse(t, example)
	sample := decl(t, f, "a")

	match := astcodec.FindFunctionsByPattern(sample.Type)

	var found []string
	assert.NotPanics(t, func() {
		for _, d := range f.Decls {
			if match(d) {
				found = append(found, d.(*ast.FuncDecl).Name.Name)
			}
		}
	}, "the example file includes a function without results")
	assert.Equal(t, []string{"a", "b"}, found)

	t.Run("functions without results", func(t *testing.T) {
		_, f, _ := parse(t, example2)
		assert.NotPanics(t, func() {
			match := astcodec.FindFunctionsByPattern(decl(t, f, "none").Type)

			var found []string
			for _, d := range f.Decls {
				if match(d) {
					found = append(found, d.(*ast.FuncDecl).Name.Name)
				}
			}
			assert.Equal(t, []string{"none", "other"}, found)
		})
	})

	t.Run("does not rename parameters of the searched functions", func(t *testing.T) {
		fset, f, _ := parse(t, example)
		before := render(t, fset, f)
		match := astcodec.FindFunctionsByPattern(decl(t, f, "a").Type)
		assert.NotPanics(t, func() {
			for _, d := range f.Decls {
				match(d)
			}
		})
		assert.Equal(t, before, render(t, fset, f))
	})
}

const example2 = `package p

func none() {}
func other() {}
func one() error { return nil }
`

func TestFindImportsByPath(t *testing.T) {
	_, f, _ := parse(t, example)
	match := astcodec.FindImportsByPath("fmt")

	var found []string
	for _, imp := range astcodec.SearchImports(f) {
		if match(imp) {
			found = append(found, imp.Path.Value)
		}
	}
	assert.Equal(t, []string{`"fmt"`}, found)
}

func TestSearchImports(t *testing.T) {
	_, f, pkg := parse(t, example)

	t.Run("no filters returns every import", func(t *testing.T) {
		assert.Len(t, astcodec.SearchImports(f), 3)
		assert.Len(t, astcodec.SearchPackageImports(pkg), 3)
	})

	t.Run("filters restrict the results", func(t *testing.T) {
		found := astcodec.SearchImports(f, astcodec.FindImportsByPath("fmt"))
		require.Len(t, found, 1)
		assert.Equal(t, `"fmt"`, found[0].Path.Value)

		assert.Len(t, astcodec.SearchPackageImports(pkg, astcodec.FindImportsByPath("fmt")), 1)
		assert.Empty(t, astcodec.SearchImports(f, astcodec.FindImportsByPath("missing")))
	})

	t.Run("only import declarations are considered", func(t *testing.T) {
		_, f, _ := parse(t, "package p\nvar v = 1\nfunc f() {}\n")
		assert.Empty(t, astcodec.SearchImports(f))
	})

	t.Run("named imports keep their name", func(t *testing.T) {
		found := astcodec.SearchImports(f, astcodec.FindImportsByPath("github.com/egdaemon/eg/runtime/wasi/eg"))
		require.Len(t, found, 1)
		require.NotNil(t, found[0].Name)
		assert.Equal(t, "eg", found[0].Name.Name)
	})
}

func TestFindImport(t *testing.T) {
	_, f, _ := parse(t, example)

	imp := astcodec.FindImport(f, astcodec.FindImportsByPath("context"))
	require.NotNil(t, imp)
	assert.Equal(t, `"context"`, imp.Path.Value)

	assert.Nil(t, astcodec.FindImport(f, astcodec.FindImportsByPath("missing")))
}

func TestSearchDecls(t *testing.T) {
	_, f, pkg := parse(t, example)

	assert.Len(t, astcodec.SearchDecls(pkg), len(f.Decls), "no filters returns every declaration")

	t.Run("filters restrict the results", func(t *testing.T) {
		found := astcodec.SearchDecls(pkg, astcodec.FindFunctionsByName("a"))
		require.Len(t, found, 1)
		assert.Equal(t, "a", found[0].(*ast.FuncDecl).Name.Name)
	})

	t.Run("only functions", func(t *testing.T) {
		assert.Len(t, astcodec.SearchDecls(pkg, astcodec.FindFunctions), 5)
	})
}

func TestFindFunctionDecl(t *testing.T) {
	_, _, pkg := parse(t, example)

	fn := astcodec.FindFunctionDecl(pkg, astcodec.FindFunctionsByName("b"))
	require.NotNil(t, fn)
	assert.Equal(t, "b", fn.Name.Name)

	assert.Nil(t, astcodec.FindFunctionDecl(pkg, astcodec.FindFunctionsByName("missing")))
	assert.Nil(t, astcodec.FindFunctionDecl(pkg, astcodec.FindFunctionsByName("v")), "variables are not functions")
}

type counter struct {
	seen int
	stop func(ast.Node) bool
}

func (t *counter) Visit(n ast.Node) ast.Visitor {
	if n == nil {
		return t
	}
	t.seen++
	if t.stop != nil && t.stop(n) {
		return nil
	}
	return t
}

func TestMultivisit(t *testing.T) {
	_, f, _ := parse(t, example)

	t.Run("every visitor sees every node", func(t *testing.T) {
		a, b := &counter{}, &counter{}
		ast.Walk(astcodec.Multivisit(a, b), f)
		assert.Positive(t, a.seen)
		assert.Equal(t, a.seen, b.seen)
	})

	t.Run("a visitor that stops descending does not break the others", func(t *testing.T) {
		all := &counter{}
		pruned := &counter{stop: func(n ast.Node) bool { _, ok := n.(*ast.FuncDecl); return ok }}

		assert.NotPanics(t, func() {
			ast.Walk(astcodec.Multivisit(all, pruned), f)
		})

		baseline := &counter{}
		ast.Walk(baseline, f)
		assert.Equal(t, baseline.seen, all.seen, "unpruned visitor still sees the full tree")
		assert.Less(t, pruned.seen, baseline.seen)
	})
}

func TestFilter(t *testing.T) {
	_, f, _ := parse(t, "package p\nfunc f() { a(b(c())) }\n")

	t.Run("only delegates matching nodes", func(t *testing.T) {
		d := &counter{}
		ast.Walk(astcodec.Filter(d, func(*ast.CallExpr) bool { return true }), f)
		assert.Equal(t, 3, d.seen)
	})

	t.Run("predicate restricts nodes", func(t *testing.T) {
		d := &counter{}
		ast.Walk(astcodec.Filter(d, func(ce *ast.CallExpr) bool { return astcodec.Ident(ce.Fun) == "b" }), f)
		assert.Equal(t, 1, d.seen)
	})

	t.Run("delegate that stops descending", func(t *testing.T) {
		d := &counter{stop: func(ast.Node) bool { return true }}
		assert.NotPanics(t, func() {
			ast.Walk(astcodec.Filter(d, func(*ast.CallExpr) bool { return true }), f)
		})
	})
}

func TestNoopAndPrinter(t *testing.T) {
	_, f, _ := parse(t, example)

	assert.NotPanics(t, func() { ast.Walk(astcodec.Noop(), f) })
	assert.NotPanics(t, func() { ast.Walk(astcodec.Printer(), f) }, "imports without a name must be printable")
}

func TestNewCallExprReplacement(t *testing.T) {
	src := "package p\nfunc f() {\n\tfoo(a)\n\tkeep(b)\n\tbar(foo(c))\n}\n"

	t.Run("replaces matching calls", func(t *testing.T) {
		fset, f, _ := parse(t, src)
		ast.Walk(astcodec.NewCallExprReplacement(
			func(ce *ast.CallExpr) *ast.CallExpr {
				return astbuild.CallExpr(ast.NewIdent("replaced"), ast.NewIdent("x"), ast.NewIdent("y"))
			},
			func(ce *ast.CallExpr) bool { return astcodec.Ident(ce.Fun) == "foo" },
		), f)

		out := render(t, fset, f)
		assert.Contains(t, out, "replaced(x, y)")
		assert.Contains(t, out, "keep(b)")
		assert.NotContains(t, out, "foo(")
		assert.Equal(t, 2, strings.Count(out, "replaced("), "nested matches are replaced as well")
	})

	t.Run("mutation returning the call is a noop", func(t *testing.T) {
		fset, f, _ := parse(t, src)
		before := render(t, fset, f)
		ast.Walk(astcodec.NewCallExprReplacement(
			func(ce *ast.CallExpr) *ast.CallExpr { return ce },
			func(ce *ast.CallExpr) bool { return true },
		), f)
		assert.Equal(t, before, render(t, fset, f))
	})

	t.Run("replacement keeps the call valid when printed", func(t *testing.T) {
		fset, f, _ := parse(t, "package p\nfunc f() {\n\tfoo(\n\t\ta,\n\t\t// comment\n\t\tb,\n\t)\n}\n")
		ast.Walk(astcodec.NewCallExprReplacement(
			func(ce *ast.CallExpr) *ast.CallExpr {
				return astbuild.CallExpr(ast.NewIdent("replaced"), ast.NewIdent("x"))
			},
			func(ce *ast.CallExpr) bool { return astcodec.Ident(ce.Fun) == "foo" },
		), f)

		out := render(t, fset, f)
		_, err := parser.ParseFile(token.NewFileSet(), "out.go", out, 0)
		assert.NoError(t, err, out)
	})
}

func TestReplaceFunction(t *testing.T) {
	fset, f, _ := parse(t, example)

	with := &ast.FuncDecl{
		Name: ast.NewIdent("main"),
		Type: &ast.FuncType{Params: astbuild.FieldList()},
		Body: &ast.BlockStmt{},
	}
	astcodec.ReplaceFunction(f, with, astcodec.FindFunctionsByName("main"))

	assert.Equal(t, []string{"main", "a", "b", "unnamed", "noresults"}, funcnames(f), "order and other functions are preserved")
	assert.Same(t, with, decl(t, f, "main"))
	assert.NotContains(t, render(t, fset, f), `"main"`, "the original body is gone")

	t.Run("no match leaves the file untouched", func(t *testing.T) {
		fset, f, _ := parse(t, example)
		before := render(t, fset, f)
		astcodec.ReplaceFunction(f, with, astcodec.FindFunctionsByName("missing"))
		assert.Equal(t, before, render(t, fset, f))
	})

	t.Run("other declarations are preserved", func(t *testing.T) {
		_, f, _ := parse(t, example)
		before := len(f.Decls)
		astcodec.ReplaceFunction(f, with, astcodec.FindFunctionsByName("main"))
		assert.Len(t, f.Decls, before)
	})
}

func TestRemoveFunction(t *testing.T) {
	_, f, _ := parse(t, example)
	before := len(f.Decls)

	astcodec.RemoveFunction(f, astcodec.FindFunctionsByName("a"))
	assert.Equal(t, []string{"main", "b", "unnamed", "noresults"}, funcnames(f))
	assert.Len(t, f.Decls, before-1)

	t.Run("multiple matches", func(t *testing.T) {
		_, f, _ := parse(t, example)
		astcodec.RemoveFunction(f, func(d ast.Decl) bool {
			fn, ok := d.(*ast.FuncDecl)
			return ok && (fn.Name.Name == "a" || fn.Name.Name == "b")
		})
		assert.Equal(t, []string{"main", "unnamed", "noresults"}, funcnames(f))
	})

	t.Run("no match", func(t *testing.T) {
		_, f, _ := parse(t, example)
		astcodec.RemoveFunction(f, astcodec.FindFunctionsByName("missing"))
		assert.Len(t, f.Decls, before)
	})
}

func TestIdent(t *testing.T) {
	assert.Equal(t, "eg.Module", astcodec.Ident(astbuild.SelExpr("eg", "Module")))
	assert.Equal(t, "x", astcodec.Ident(ast.NewIdent("x")))
	assert.Equal(t, "f(a)", astcodec.Ident(astbuild.CallExpr(ast.NewIdent("f"), ast.NewIdent("a"))))
}

func TestFormat(t *testing.T) {
	t.Run("formats source", func(t *testing.T) {
		out, err := astcodec.Format("package p\nfunc   f( ) {\nreturn\n}\n")
		require.NoError(t, err)
		assert.Equal(t, "package p\n\nfunc f() {\n\treturn\n}\n", out)
	})

	t.Run("fragment", func(t *testing.T) {
		out, err := astcodec.Format("x   :=   1")
		require.NoError(t, err)
		assert.Equal(t, "x := 1", out)
	})

	t.Run("adds missing imports", func(t *testing.T) {
		out, err := astcodec.Format("package p\nfunc f() { fmt.Println(\"x\") }\n")
		require.NoError(t, err)
		assert.Contains(t, out, `import "fmt"`)
	})

	t.Run("invalid source", func(t *testing.T) {
		_, err := astcodec.Format("package p\nfunc f( {\n")
		assert.Error(t, err)
	})

	t.Run("comments are preserved", func(t *testing.T) {
		out, err := astcodec.Format("package p\n\n// doc\nfunc f() {}\n")
		require.NoError(t, err)
		assert.Contains(t, out, "// doc")
	})
}

func TestReformat(t *testing.T) {
	write := func(t *testing.T, content string) *os.File {
		f, err := os.CreateTemp(t.TempDir(), "*.go")
		require.NoError(t, err)
		t.Cleanup(func() { f.Close() })
		_, err = f.WriteString(content)
		require.NoError(t, err)
		return f
	}

	read := func(t *testing.T, f *os.File) string {
		b, err := os.ReadFile(f.Name())
		require.NoError(t, err)
		return string(b)
	}

	t.Run("formats and adds imports", func(t *testing.T) {
		f := write(t, "package p\nfunc   f( ) { fmt.Println(\"x\") }\n")
		require.NoError(t, astcodec.Reformat(f))
		assert.Equal(t, "package p\n\nimport \"fmt\"\n\nfunc f() { fmt.Println(\"x\") }\n", read(t, f))
	})

	t.Run("shrinking content truncates the file", func(t *testing.T) {
		long := "package p\n\n// " + strings.Repeat("padding ", 50) + "\nfunc   f( ) {}\n"
		f := write(t, long)
		require.NoError(t, astcodec.Reformat(f))
		out := read(t, f)
		assert.True(t, strings.HasSuffix(out, "func f() {}\n"), out)
	})

	t.Run("invalid source leaves the file untouched", func(t *testing.T) {
		f := write(t, "package p\nfunc f( {\n")
		assert.Error(t, astcodec.Reformat(f))
		assert.Equal(t, "package p\nfunc f( {\n", read(t, f))
	})

	t.Run("starts from the beginning when the offset is moved", func(t *testing.T) {
		f := write(t, "package p\nfunc   f( ) {}\n")
		_, err := f.Seek(0, 2)
		require.NoError(t, err)
		require.NoError(t, astcodec.Reformat(f))
		assert.Contains(t, read(t, f), "func f() {}")
	})
}

func TestLoadOptions(t *testing.T) {
	cfg := astcodec.DefaultPkgLoad()
	assert.NotEmpty(t, cfg.Env, "defaults to the current environment")
	assert.NotZero(t, cfg.Mode&packages.NeedSyntax)
	assert.NotZero(t, cfg.Mode&packages.NeedTypes)
	assert.NotZero(t, cfg.Mode&packages.NeedModule)

	dir := t.TempDir()
	cfg = astcodec.DefaultPkgLoad(astcodec.LoadDir(dir), astcodec.AutoFileSet)
	assert.Equal(t, dir, cfg.Dir)
	assert.NotNil(t, cfg.Fset)

	t.Run("disable gowork", func(t *testing.T) {
		cfg := astcodec.DefaultPkgLoad(astcodec.DisableGowork)
		assert.Equal(t, "GOWORK=off", cfg.Env[len(cfg.Env)-1])
	})

	t.Run("options do not leak between configs", func(t *testing.T) {
		astcodec.DefaultPkgLoad(astcodec.DisableGowork)
		other := astcodec.DefaultPkgLoad()
		for _, e := range other.Env {
			if strings.HasPrefix(e, "GOWORK=") {
				assert.Equal(t, os.Getenv("GOWORK"), strings.TrimPrefix(e, "GOWORK="))
			}
		}
	})
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/x\n\ngo 1.21\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0600))

	pkg, err := astcodec.Load(astcodec.DefaultPkgLoad(astcodec.LoadDir(dir), astcodec.AutoFileSet))
	require.NoError(t, err)
	require.NotNil(t, pkg)
	assert.Equal(t, "main", pkg.Name)
	assert.Equal(t, "example.com/x", pkg.PkgPath)
	require.Len(t, pkg.Syntax, 1)
	assert.NotNil(t, astcodec.FindFunctionDecl(pkg, astcodec.FindFunctionsByName("main")))

	t.Run("directory without go files", func(t *testing.T) {
		empty := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(empty, "go.mod"), []byte("module example.com/y\n\ngo 1.21\n"), 0600))
		// package errors are intentionally left for the caller to inspect.
		pkg, err := astcodec.Load(astcodec.DefaultPkgLoad(astcodec.LoadDir(empty), astcodec.AutoFileSet))
		require.NoError(t, err)
		require.NotNil(t, pkg)
		assert.Empty(t, pkg.Syntax)
	})
}
