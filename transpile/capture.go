package transpile

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"sort"
	"strings"

	"github.com/dave/jennifer/jen"
)

// renderModule renders the generated source. jennifer discards its formatting when the source
// is invalid and reports the raw unformatted text, so format here and report failures with line numbers.
func renderModule(f *jen.File) ([]byte, error) {
	f.NoFormat = true
	raw := bytes.NewBuffer(nil)
	if err := f.Render(raw); err != nil {
		return nil, err
	}

	formatted, err := format.Source(raw.Bytes())
	if err != nil {
		var b strings.Builder
		for i, line := range strings.Split(raw.String(), "\n") {
			fmt.Fprintf(&b, "%4d | %s\n", i+1, line)
		}
		return nil, fmt.Errorf("generated module is invalid: %w\n%s", err, b.String())
	}

	return formatted, nil
}

// generated modules declare these themselves, statements that exclusively declare
// them are never copied.
var generatedIdents = map[string]bool{"ctx": true, "done": true}

func render(fset *token.FileSet, n ast.Node) (string, error) {
	buf := bytes.NewBuffer(nil)
	if err := printer.Fprint(buf, fset, n); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func within(outer, inner ast.Node) bool {
	return outer.Pos() <= inner.Pos() && inner.End() <= outer.End()
}

// enclosingModuleCalls maps each call matching pattern to the function declaration containing it.
func enclosingCalls(c *ast.File, pattern func(*ast.CallExpr) bool) map[*ast.CallExpr]*ast.FuncDecl {
	m := make(map[*ast.CallExpr]*ast.FuncDecl)
	for _, d := range c.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if ce, ok := n.(*ast.CallExpr); ok && pattern(ce) {
				m[ce] = fn
			}
			return true
		})
	}
	return m
}

func declaresOnly(s ast.Stmt, names map[string]bool) bool {
	var declared []*ast.Ident
	switch x := s.(type) {
	case *ast.AssignStmt:
		for _, l := range x.Lhs {
			if id, ok := l.(*ast.Ident); ok {
				declared = append(declared, id)
			}
		}
	case *ast.DeclStmt:
		if gd, ok := x.Decl.(*ast.GenDecl); ok {
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					declared = append(declared, vs.Names...)
				}
			}
		}
	}
	for _, id := range declared {
		if !names[id.Name] {
			return false
		}
	}
	return len(declared) > 0
}

// capturedStatements finds the statements of the function body that declare the local variables,
// constants and types referenced by the ops (transitively) and returns them in source order.
func capturedStatements(body *ast.BlockStmt, call *ast.CallExpr, ops []ast.Expr) ([]ast.Stmt, error) {
	// index of the top level statement containing the node.
	toplevel := func(n ast.Node) int {
		for i, s := range body.List {
			if within(s, n) {
				return i
			}
		}
		return -1
	}

	selected := make(map[int]bool)
	var pending []ast.Node

	scan := func(root ast.Node, self int) error {
		var err error
		ast.Inspect(root, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || id.Obj == nil || err != nil {
				return err == nil
			}

			switch id.Obj.Kind {
			case ast.Var, ast.Con, ast.Typ:
			default:
				return true
			}

			decl, ok := id.Obj.Decl.(ast.Node)
			if !ok || decl.Pos() < body.Pos() || decl.End() > body.End() {
				return true // not local to the function.
			}

			// declared within the node being scanned, e.g. closure parameters.
			if within(root, decl) {
				return true
			}

			idx := toplevel(decl)
			if idx < 0 || idx == self || selected[idx] {
				return true
			}

			stmt := body.List[idx]
			switch stmt.(type) {
			case *ast.DeclStmt, *ast.AssignStmt:
			default:
				err = fmt.Errorf("unable to transfer %s into the generated module: it is declared by a %T", id.Name, stmt)
				return false
			}

			if declaresOnly(stmt, generatedIdents) {
				return true
			}

			if within(stmt, call) {
				err = fmt.Errorf("unable to transfer %s into the generated module: its declaration contains the module", id.Name)
				return false
			}

			selected[idx] = true
			pending = append(pending, stmt)
			return true
		})
		return err
	}

	for _, op := range ops {
		if err := scan(op, -1); err != nil {
			return nil, err
		}
	}

	drain := func() error {
		for len(pending) > 0 {
			s := pending[0]
			pending = pending[1:]
			if err := scan(s, toplevel(s)); err != nil {
				return err
			}
		}
		return nil
	}

	if err := drain(); err != nil {
		return nil, err
	}

	// assignments to captured variables preceding the module are transferred as well,
	// i.e. `var fn func(); fn = func() { fn() }`.
	callidx := toplevel(call)
	assigns := func(s ast.Stmt) bool {
		var targets []ast.Expr
		switch x := s.(type) {
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE {
				return false
			}
			targets = x.Lhs
		case *ast.IncDecStmt:
			targets = []ast.Expr{x.X}
		default:
			return false
		}

		for _, t := range targets {
			if id := rootIdent(t); id != nil && id.Obj != nil {
				if decl, ok := id.Obj.Decl.(ast.Node); ok && selected[toplevel(decl)] {
					return true
				}
			}
		}
		return false
	}

	for changed := true; changed; {
		changed = false
		for i, s := range body.List {
			if selected[i] || (callidx >= 0 && i >= callidx) || !assigns(s) {
				continue
			}
			selected[i] = true
			pending = append(pending, s)
			changed = true
		}
		if err := drain(); err != nil {
			return nil, err
		}
	}

	idxs := make([]int, 0, len(selected))
	for i := range selected {
		idxs = append(idxs, i)
	}
	sort.Ints(idxs)

	stmts := make([]ast.Stmt, 0, len(idxs))
	for _, i := range idxs {
		stmts = append(stmts, body.List[i])
	}
	return stmts, nil
}

// declaredVars returns the names of the variables (not constants or types) declared by the statement.
func declaredVars(s ast.Stmt) (names []string) {
	switch x := s.(type) {
	case *ast.AssignStmt:
		if x.Tok != token.DEFINE {
			return nil
		}
		for _, l := range x.Lhs {
			if id, ok := l.(*ast.Ident); ok && id.Name != "_" {
				names = append(names, id.Name)
			}
		}
	case *ast.DeclStmt:
		if gd, ok := x.Decl.(*ast.GenDecl); ok && gd.Tok == token.VAR {
			for _, spec := range gd.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for _, id := range vs.Names {
						if id.Name != "_" {
							names = append(names, id.Name)
						}
					}
				}
			}
		}
	}
	return names
}

// markUsed inserts `_ = name` after each captured variable declaration. The module call is replaced
// in the original function, which may have been the only use of the variable and go rejects unused variables.
func markUsed(body *ast.BlockStmt, captured map[ast.Stmt]struct{}) {
	if len(captured) == 0 {
		return
	}

	stmts := make([]ast.Stmt, 0, len(body.List)+len(captured))
	for _, s := range body.List {
		stmts = append(stmts, s)
		if _, ok := captured[s]; !ok {
			continue
		}
		for _, name := range declaredVars(s) {
			stmts = append(stmts, &ast.AssignStmt{
				Lhs: []ast.Expr{ast.NewIdent("_")},
				Tok: token.ASSIGN,
				Rhs: []ast.Expr{ast.NewIdent(name)},
			})
		}
	}
	body.List = stmts
}

func rootIdent(e ast.Expr) *ast.Ident {
	for {
		switch x := e.(type) {
		case *ast.Ident:
			return x
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		case *ast.ParenExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// unusedVars returns the variables declared by the statements that nothing reads, go rejects
// unused variables and copying a statement can leave some of the names it declares without a use.
func unusedVars(stmts []ast.Stmt, ops []ast.Expr) (names []string) {
	skip := make(map[*ast.Ident]bool)
	used := make(map[*ast.Object]bool)

	visit := func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				if id, ok := l.(*ast.Ident); ok {
					skip[id] = true
				}
			}
		case *ast.ValueSpec:
			for _, id := range x.Names {
				skip[id] = true
			}
		case *ast.Ident:
			if x.Obj != nil && !skip[x] {
				used[x.Obj] = true
			}
		}
		return true
	}

	for _, s := range stmts {
		ast.Inspect(s, visit)
	}
	for _, op := range ops {
		ast.Inspect(op, visit)
	}

	for _, s := range stmts {
		ast.Inspect(s, func(n ast.Node) bool {
			var ids []*ast.Ident
			switch x := n.(type) {
			case *ast.AssignStmt:
				if x.Tok != token.DEFINE {
					return true
				}
				for _, l := range x.Lhs {
					if id, ok := l.(*ast.Ident); ok {
						ids = append(ids, id)
					}
				}
			case *ast.GenDecl:
				if x.Tok != token.VAR {
					return true
				}
				for _, spec := range x.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						ids = append(ids, vs.Names...)
					}
				}
			}
			for _, id := range ids {
				if id.Name != "_" && id.Obj != nil && !used[id.Obj] {
					names = append(names, id.Name)
				}
			}
			return true
		})
	}

	return names
}
