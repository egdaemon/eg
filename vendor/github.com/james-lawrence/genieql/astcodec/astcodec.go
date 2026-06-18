package astcodec

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log"

	"github.com/james-lawrence/genieql/astbuild"
	"github.com/james-lawrence/genieql/internal/errorsx"
	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

// ErrPackageNotFound returned when the requested package cannot be located
// within the given context.
const ErrPackageNotFound = errorsx.String("package not found")

type LoadOpt func(*packages.Config)

func LoadDir(path string) LoadOpt {
	return func(c *packages.Config) {
		c.Dir = path
	}
}

func AutoFileSet(c *packages.Config) {
	c.Fset = token.NewFileSet()
}

func LocatePackages(options ...LoadOpt) *packages.Config {
	cfg := &packages.Config{
		Fset: token.NewFileSet(),
		Mode: packages.NeedName | packages.LoadImports,
	}

	for _, opt := range options {
		opt(cfg)
	}

	return cfg
}

func DefaultPkgLoad(options ...LoadOpt) *packages.Config {
	cfg := &packages.Config{
		Fset: token.NewFileSet(),
		Mode: packages.NeedName | packages.NeedSyntax,
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

func LoadFirst(cfg *packages.Config) (pkg *packages.Package, err error) {
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

func FilterFunctionsByName(n string) func(d ast.Decl) bool {
	return func(d ast.Decl) bool {
		fn, ok := d.(*ast.FuncDecl)
		if !ok {
			return ok
		}

		return fn.Name.Name != n
	}
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

func ReplaceFunctionBody(body *ast.BlockStmt) func(fn *ast.FuncDecl) *ast.FuncDecl {
	return func(fd *ast.FuncDecl) *ast.FuncDecl {
		return &ast.FuncDecl{
			Doc:  fd.Doc,
			Recv: fd.Recv,
			Name: fd.Name,
			Type: fd.Type,
			Body: body,
		}
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

func FilterImports(n ast.Decl) bool {
	if d, ok := n.(*ast.GenDecl); ok {
		return d.Tok == token.IMPORT
	}

	return false
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

func SearchFileDecls(gf *ast.File, filters ...func(ast.Decl) bool) (fn []ast.Decl) {
	match := func(d ast.Decl) bool {
		for _, f := range filters {
			if f(d) {
				return true
			}
		}

		return false
	}

	for _, d := range gf.Decls {
		if match(d) {
			fn = append(fn, d)
			continue
		}
	}

	return fn
}

func FileFindDecl[T ast.Node](gf *ast.File, filters ...func(ast.Decl) bool) (fn T) {
	match := func(d ast.Decl) bool {
		for _, f := range filters {
			if f(d) {
				return true
			}
		}

		return false
	}

	for _, d := range gf.Decls {
		if match(d) {
			return d.(T)
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
	case *ast.File:
		return t
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
	case *ast.GenDecl:
		log.Printf("%T - %s\n", x, x.Tok)
	case *ast.ImportSpec:
		log.Printf("%T - %s\n", x, x.Path.Value)
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

func NewFunctionReplacement(mut func(*ast.FuncDecl) *ast.FuncDecl, pattern func(ast.Decl) bool) ast.Visitor {
	return replacefunction{
		mut:     mut,
		pattern: pattern,
	}
}

func NewIdentReplacement(mut func(*ast.Ident) *ast.Ident, pattern func(*ast.Ident) bool) ast.Visitor {
	return replaceidentexpr{
		mut:     mut,
		pattern: pattern,
	}
}

type replaceidentexpr struct {
	pattern func(*ast.Ident) bool
	mut     func(*ast.Ident) *ast.Ident
}

func (t replaceidentexpr) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return t
	}

	switch x := node.(type) {
	case *ast.Ident:
		if t.pattern(x) {
			replacement := t.mut(x)
			x.Name = replacement.Name
		}
	default:
		// log.Printf("%T\n", x)
	}

	return t
}

func NewEnsureImport(i string) ast.Visitor {
	return ensureimport{
		importname: i,
	}
}

type ensureimport struct {
	importname string
}

func (t ensureimport) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return t
	}
	switch x := node.(type) {
	case *ast.GenDecl:
		EnsureImport(x, t.importname)
	default:
		// log.Printf("%T\n", x)
	}
	return t
}

func NewRemoveImport(i string) ast.Visitor {
	return removeimport{
		importname: i,
	}
}

type removeimport struct {
	importname string
}

func (t removeimport) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return t
	}
	switch x := node.(type) {
	case *ast.GenDecl:
		RemoveImport(x, t.importname)
	default:
		// log.Printf("%T\n", x)
	}
	return t
}

type replacefunction struct {
	pattern func(ast.Decl) bool
	mut     func(*ast.FuncDecl) *ast.FuncDecl
}

func (t replacefunction) Visit(node ast.Node) (w ast.Visitor) {
	if node == nil {
		return t
	}

	switch x := node.(type) {
	case *ast.FuncDecl:
		if t.pattern(x) {
			replacement := t.mut(x)
			*x = *replacement
		}
	default:
		// log.Printf("%T\n", x)
	}

	return t
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

func RemoveImport(root ast.Node, n string) ast.Node {
	return astutil.Apply(root, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.File:
			return true
		case *ast.GenDecl:
			return n.Tok.String() == "import"
		case *ast.ImportSpec:
			return true
		default:
			return false
		}
	}, func(c *astutil.Cursor) bool {
		node, ok := c.Node().(*ast.ImportSpec)
		if !ok {
			return true
		}

		if node.Path.Value == fmt.Sprintf("\"%s\"", n) {
			c.Delete()
		}

		return true
	})
}

func EnsureImport(root ast.Node, n string) ast.Node {
	return astutil.Apply(root, func(c *astutil.Cursor) bool {
		switch n := c.Node().(type) {
		case *ast.File:
			return true
		case *ast.GenDecl:
			return n.Tok.String() == "import"
		default:
			return false
		}
	}, func(c *astutil.Cursor) bool {
		node, ok := c.Node().(*ast.GenDecl)
		if !ok {
			return true
		}

		node.Specs = append(node.Specs, astbuild.ImportSpecLiteral(nil, n))
		return true
	})
}

func PrintImports(tree *ast.File) {
	for _, i := range tree.Imports {
		log.Println("import", i.Path.Value)
	}
}

func Ident(expr ast.Expr) string {
	return types.ExprString(expr)
}
