package transpile

import (
	"context"
	"go/ast"
	"go/printer"
	"go/types"
	"os"
	"path/filepath"

	"github.com/james-lawrence/eg/astbuild"
	"github.com/james-lawrence/eg/astcodec"
	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/workspaces"
	"github.com/pkg/errors"
	"golang.org/x/tools/go/packages"
)

type Context struct {
	Workspace workspaces.Context
}

type Transpiler interface {
	Run(ctx context.Context, tctx Context) (roots []string, err error)
}

func New(ws workspaces.Context) Context {
	return Context{
		Workspace: ws,
	}
}

// Autodetect the transpiler to use.
func Autodetect() Transpiler {
	return golang{}
}

const Skip = errorsx.String("skipping content")

type golang struct{}

func (t golang) Run(ctx context.Context, tctx Context) (roots []string, err error) {
	var (
		dst      string
		pkg      *packages.Package
		yakident = "yak"
	)

	pkgc := astcodec.DefaultPkgLoad(
		astcodec.LoadDir(filepath.Join(tctx.Workspace.Root, tctx.Workspace.ModuleDir)),
		astcodec.AutoFileSet,
	)

	if pkg, err = astcodec.Load(pkgc, "eg/ci"); err != nil {
		return roots, errors.Wrap(err, "unable to load package eg/ci")
	}

	if imp := astcodec.FindImport(pkg, astcodec.FindImportsByPath("github.com/james-lawrence/eg/runtime/wasi/yak")); imp != nil {
		yakident = imp.Name.String()
	}
	refexpr := astbuild.SelExpr(yakident, "Ref")

	for _, c := range pkg.Syntax {
		ast.Walk(astcodec.NewCallExprReplacement(func(ce *ast.CallExpr) *ast.CallExpr {
			args := []ast.Expr{
				astbuild.StringLiteral(types.ExprString(ce.Args[0])),
			}
			args = append(args, ce.Args...)
			return astbuild.CallExpr(astbuild.SelExpr(yakident, "UnsafeTranspiledRef"), args...)
		}, func(ce *ast.CallExpr) bool {
			return astcodec.TypePattern(refexpr)(ce.Fun)
		}), c)
	}

	for _, c := range pkg.Syntax {
		var (
			iodst *os.File
		)

		ftoken := pkg.Fset.File(c.Pos())
		if dst, err = transpiledpath(tctx, filepath.Join(tctx.Workspace.Root, tctx.Workspace.ModuleDir), ftoken.Name()); err != nil {
			return roots, err
		}

		if err = os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return roots, err
		}

		if iodst, err = os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600); err != nil {
			return roots, err
		}
		defer iodst.Close()

		if err = printer.Fprint(iodst, pkg.Fset, c); err != nil {
			return roots, err
		}

		if mainfn := astcodec.FindFunctionDecl(pkg, astcodec.FindFunctionsByName("main")); mainfn != nil {
			roots = append(roots, dst)
		}
	}

	return roots, nil
}

func transpiledpath(tctx Context, mdir string, current string) (path string, err error) {
	if path, err = filepath.Rel(mdir, current); err != nil {
		return "", err
	}
	return filepath.Join(tctx.Workspace.TransDir, path), nil
}
