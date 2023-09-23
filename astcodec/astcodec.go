package astcodec

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"golang.org/x/tools/go/ast/astutil"
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
		Mode: packages.NeedName | packages.NeedImports | packages.NeedTypes | packages.NeedSyntax | packages.NeedFiles | packages.NeedDeps | packages.NeedModule,
	}

	for _, opt := range options {
		opt(cfg)
	}

	return cfg
}

func Load(cfg *packages.Config) (pkg *packages.Package, err error) {
	var set []*packages.Package

	if set, err = packages.Load(cfg); err != nil {
		return nil, err
	}

	for _, pkg = range set {
		return pkg, nil
	}

	return nil, errors.New("no package found")
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

func SearchPackageImports(pkg *packages.Package, filters ...func(*ast.ImportSpec) bool) (fn []*ast.ImportSpec) {
	for _, gf := range pkg.Syntax {
		fn = append(fn, SearchImports(gf, filters...)...)
	}

	return fn
}

func SearchImports(root ast.Node, filters ...func(*ast.ImportSpec) bool) (fn []*ast.ImportSpec) {
	x := &findimports{}

	ast.Walk(x, root)

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

func FindImport(root ast.Node, filters ...func(*ast.ImportSpec) bool) *ast.ImportSpec {
	found := SearchImports(root, filters...)
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

type multivisit []ast.Visitor

func (t multivisit) Visit(node ast.Node) (w ast.Visitor) {
	updates := make([]ast.Visitor, 0, len(t))
	for _, v := range t {
		updates = append(updates, v.Visit(node))
	}

	return multivisit(updates)
}

func Multivisit(set ...ast.Visitor) ast.Visitor {
	return multivisit(set)
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

func Printer() ast.Visitor {
	return nodePrinter{}
}

type nodePrinter struct{}

func (t nodePrinter) Visit(node ast.Node) (w ast.Visitor) {
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

type filter struct {
	delegate ast.Visitor
	pattern  func(ast.Node) bool
}

func (t filter) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return t
	}

	if !t.pattern(node) {
		return t
	}

	return filter{
		delegate: t.delegate.Visit(node),
		pattern:  t.pattern,
	}
}

func Filter[T ast.Node](v ast.Visitor, m func(T) bool) ast.Visitor {
	return filter{
		delegate: v,
		pattern: func(n ast.Node) bool {
			switch x := n.(type) {
			case T:
				return m(x)
			default:
				return false
			}
		},
	}
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

func ReplaceFunction(root ast.Node, with *ast.FuncDecl, pattern func(ast.Decl) bool) ast.Node {
	return astutil.Apply(root, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.File:
			return true
		case *ast.FuncDecl:
			return pattern(n)
		case ast.Decl:
			return false
		default:
			return false
		}
	}, func(c *astutil.Cursor) bool {
		if _, ok := c.Node().(*ast.FuncDecl); !ok {
			return true
		}
		c.InsertAfter(with)
		c.Delete()
		return true
	})
}

func RemoveFunction(root ast.Node, pattern func(ast.Decl) bool) ast.Node {
	return astutil.Apply(root, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.File:
			return true
		case *ast.FuncDecl:
			return pattern(n)
		case ast.Decl:
			return false
		default:
			return false
		}
	}, func(c *astutil.Cursor) bool {
		if _, ok := c.Node().(*ast.FuncDecl); !ok {
			return true
		}
		c.Delete()
		return true
	})
}

func Ident(expr ast.Expr) string {
	return types.ExprString(expr)
}
