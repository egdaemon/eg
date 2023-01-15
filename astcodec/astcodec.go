package astcodec

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"golang.org/x/tools/go/packages"
)

type LoadOpt func(*packages.Config)

func LoadDir(path string) LoadOpt {
	return func(c *packages.Config) {
		c.Dir = path
	}
}

func AutoFileSet(c *packages.Config) {
	c.Fset = token.NewFileSet()
}

func DefaultPkgLoad(options ...LoadOpt) *packages.Config {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedImports | packages.NeedTypes | packages.NeedSyntax | packages.NeedFiles | packages.NeedDeps,
	}

	for _, opt := range options {
		opt(cfg)
	}

	return cfg
}

func Load(cfg *packages.Config, name string) (pkg *packages.Package, err error) {
	var set []*packages.Package

	if set, err = packages.Load(cfg, name); err != nil {
		return nil, err
	}

	return set[0], nil
}

func FindFunctions(d ast.Decl) bool {
	_, ok := d.(*ast.FuncDecl)
	return ok
}

func FindFunctionsByName(n string) func(d ast.Decl) bool {
	return func(d ast.Decl) bool {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			return ok
		}

		return fn.Name.Name == n
	}
}

func TypePattern(pattern ...ast.Expr) func(...ast.Expr) bool {
	return func(testcase ...ast.Expr) bool {
		if len(pattern) != len(testcase) {
			return false
		}

		for idx := range pattern {
			if types.ExprString(pattern[idx]) != types.ExprString(testcase[idx]) {
				return false
			}
		}

		return true
	}
}

// MapFieldsToTypeExpr - extracts the type for each name for each of the provided fields.
// i.e.) a,b int, c string, d float is transformed into: int, int, string, float
func MapFieldsToTypeExpr(args ...*ast.Field) []ast.Expr {
	r := []ast.Expr{}
	for idx, f := range args {
		if len(f.Names) == 0 {
			f.Names = []*ast.Ident{ast.NewIdent(fmt.Sprintf("f%d", idx))}
		}

		for range f.Names {
			r = append(r, f.Type)
		}

	}
	return r
}

func FieldListPattern(l *ast.FieldList) []ast.Expr {
	return MapFieldsToTypeExpr(l.List...)
}

func FunctionPattern(example *ast.FuncType) (params []ast.Expr, results []ast.Expr) {
	return FieldListPattern(example.Params), FieldListPattern(example.Results)
}

func FindFunctionsByPattern(example *ast.FuncType) func(d ast.Decl) bool {
	paramspattern, resultpattern := FunctionPattern(example)
	return func(d ast.Decl) bool {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			return ok
		}

		aparamspattern, aresultpattern := FunctionPattern(fn.Type)

		return TypePattern(paramspattern...)(aparamspattern...) && TypePattern(resultpattern...)(aresultpattern...)
	}
}

func FindImportsByPath(path string) func(*ast.ImportSpec) bool {
	path = "\"" + path + "\""
	return func(n *ast.ImportSpec) bool {
		return path == n.Path.Value
	}
}

func SearchDecls(pkg *packages.Package, filters ...func(ast.Decl) bool) (fn []ast.Decl) {
	for _, gf := range pkg.Syntax {
		for _, d := range gf.Decls {
			for _, f := range filters {
				if !f(d) {
					continue
				}
			}

			fn = append(fn, d)
		}
	}

	return fn
}

func SearchImports(pkg *packages.Package, filters ...func(*ast.ImportSpec) bool) (fn []*ast.ImportSpec) {
	for _, gf := range pkg.Syntax {
		for _, d := range gf.Decls {
			x := &findimports{}
			ast.Walk(x, d)

			for _, s := range x.found {
				for _, f := range filters {
					if !f(s) {
						continue
					}
				}

				fn = append(fn, s)
			}

			return fn
		}
	}

	return fn
}

func FindImport(pkg *packages.Package, filters ...func(*ast.ImportSpec) bool) *ast.ImportSpec {
	found := SearchImports(pkg, filters...)
	for _, i := range found {
		for _, f := range filters {
			if f(i) {
				return i
			}
		}
	}

	return nil
}

func FindFunctionDecl(pkg *packages.Package, filters ...func(ast.Decl) bool) *ast.FuncDecl {
	found := SearchDecls(pkg, filters...)
	for _, i := range found {
		for _, f := range filters {
			if x, ok := i.(*ast.FuncDecl); ok && f(i) {
				return x
			}
		}
	}

	return nil
}

type findimports struct {
	found []*ast.ImportSpec
}

func (t *findimports) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return t
	}

	switch x := node.(type) {
	case *ast.GenDecl:
		return t
	case *ast.ImportSpec:
		t.found = append(t.found, x)
		return nil
	default:
	}

	return nil
}

type PrintNodes struct{}

func (t PrintNodes) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return t
	}

	switch x := node.(type) {
	case *ast.CallExpr:
		log.Println("invocation of", types.ExprString(x.Fun))
	default:
		log.Printf("%T\n", x)
	}
	return t
}

type replacecallexpr struct {
	pattern func(*ast.CallExpr) bool
	mut     func(*ast.CallExpr) *ast.CallExpr
}

func (t replacecallexpr) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return t
	}

	switch x := node.(type) {
	case *ast.CallExpr:
		if t.pattern(x) {
			replacement := t.mut(x)
			x.Args = replacement.Args
			x.Fun = replacement.Fun
		}
	default:
		// log.Printf("%T\n", x)
	}

	return t
}

func NewCallExprReplacement(mut func(*ast.CallExpr) *ast.CallExpr, pattern func(*ast.CallExpr) bool) ast.Visitor {
	return replacecallexpr{
		mut:     mut,
		pattern: pattern,
	}
}
