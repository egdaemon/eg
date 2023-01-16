package transpile

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/james-lawrence/eg/astbuild"
	"github.com/james-lawrence/eg/astcodec"
	"github.com/james-lawrence/eg/workspaces"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/packages"
)

type Context struct {
	Workspace workspaces.Context
}

type Transpiler interface {
	Run(ctx context.Context) (roots []string, err error)
}

func New(ws workspaces.Context) Context {
	return Context{
		Workspace: ws,
	}
}

// Autodetect the transpiler to use.
func Autodetect(tctx Context) Transpiler {
	return golang{Context: tctx}
}

type golang struct {
	Context
}

func (t golang) Run(ctx context.Context) (roots []string, err error) {
	type module struct {
		fname    string
		original *bytes.Buffer
		main     *bytes.Buffer
		pos      token.Position
	}

	var (
		dst      string
		pkg      *packages.Package
		yakident = "yak"
	)

	pkgc := astcodec.DefaultPkgLoad(
		astcodec.LoadDir(filepath.Join(t.Context.Workspace.Root, t.Context.Workspace.ModuleDir)),
		astcodec.AutoFileSet,
	)

	if pkg, err = astcodec.Load(pkgc, "eg/ci"); err != nil {
		return roots, errors.Wrap(err, "unable to load package eg/ci")
	}

	if imp := astcodec.FindImport(pkg, astcodec.FindImportsByPath("github.com/james-lawrence/eg/runtime/wasi/yak")); imp != nil {
		yakident = imp.Name.String()
	}
	refexpr := astbuild.SelExpr(yakident, "Ref")
	refmodule := astbuild.SelExpr(yakident, "Module")

	generatedmodules := make([]*module, 0, 128)

	transform := func(ftoken *token.File, gendir string, c *ast.File) error {
		// make a clone of the buffer
		buf := bytes.NewBuffer(nil)
		if err = printer.Fprint(buf, pkg.Fset, c); err != nil {
			return err
		}

		ast.Walk(astcodec.NewCallExprReplacement(func(ce *ast.CallExpr) *ast.CallExpr {
			args := []ast.Expr{
				astbuild.StringLiteral(types.ExprString(ce.Args[0])),
			}
			args = append(args, ce.Args...)
			return astbuild.CallExpr(astbuild.SelExpr(yakident, "UnsafeTranspiledRef"), args...)
		}, func(ce *ast.CallExpr) bool {
			return astcodec.TypePattern(refexpr)(ce.Fun)
		}), c)

		ast.Walk(astcodec.NewCallExprReplacement(func(ce *ast.CallExpr) *ast.CallExpr {
			log.Println("replacing moduleref", types.ExprString(ce))
			if len(ce.Args) < 3 {
				log.Println("unable to transpile module call")
				return ce
			}

			statements := make([]jen.Code, 0, len(ce.Args[2:]))
			for _, op := range ce.Args[2:] {
				statements = append(statements, jen.Id(types.ExprString(op)))
			}

			pos := pkg.Fset.PositionFor(ce.Pos(), true)
			pos.Filename = strings.TrimPrefix(pos.Filename, t.Workspace.Root+"/")

			main := jen.NewFile("main")
			main.Commentf("automatically generated from: %s", pos)
			main.Func().Id("main").Params().Block(
				jen.If(
					jen.Id("err").Op(":=").Add(jen.Qual(yakident, "Perform").Call(
						jen.Id("context.Background()"),
						jen.Qual(yakident, "Sequential").Call(statements...)),
					),
					jen.Id("err").Op("!=").Id("nil"),
				).Block(
					jen.Panic(jen.Id("err")),
				),
			)
			generatedmodules = append(generatedmodules, &module{
				fname:    filepath.Join(gendir, fmt.Sprintf("module.%d.%d.go", pos.Line, pos.Column)),
				original: buf,
				main:     bytes.NewBufferString(fmt.Sprintf("%#v", main)),
				pos:      pkg.Fset.PositionFor(ce.Pos(), true),
			})

			// log.Println("derp fn", types.ExprString(astbuild.CallExpr(ast.NewIdent("derp"), ops...)))

			// TODO: replace yak.Module with a call that points to a cached file.
			return ce
		}, func(ce *ast.CallExpr) bool {
			return astcodec.TypePattern(refmodule)(ce.Fun)
		}), c)

		return nil
	}

	rewrite := func(ftoken *token.File, dst string, c ast.Node) error {
		var (
			iodst     *os.File
			formatted string
			buf       = bytes.NewBuffer(nil)
		)

		if err = (&printer.Config{Mode: printer.RawFormat}).Fprint(buf, pkg.Fset, c); err != nil {
			return err
		}

		if formatted, err = astcodec.Format(buf.String()); err != nil {
			return err
		}

		if err = os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return err
		}

		if iodst, err = os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600); err != nil {
			return err
		}
		defer iodst.Close()

		if _, err := io.Copy(iodst, bytes.NewBufferString(formatted)); err != nil {
			return err
		}

		return nil
	}

	for _, c := range pkg.Syntax {
		var (
			gendir string
		)
		ftoken := pkg.Fset.File(c.Pos())

		if dst, err = workspaces.PathTranspiled(t.Context.Workspace, filepath.Join(t.Context.Workspace.Root, t.Context.Workspace.ModuleDir), ftoken.Name()); err != nil {
			return roots, err
		}

		if gendir, err = workspaces.PathGenMod(t.Context.Workspace, filepath.Join(t.Context.Workspace.Root, t.Context.Workspace.ModuleDir), workspaces.ReplaceExt(ftoken.Name(), ".wasm.d")); err != nil {
			return roots, err
		}

		if err = transform(ftoken, gendir, c); err != nil {
			return roots, err
		}

		if err = rewrite(ftoken, dst, c); err != nil {
			return roots, err
		}

		if mainfn := astcodec.FindFunctionDecl(pkg, astcodec.FindFunctionsByName("main")); mainfn != nil {
			roots = append(roots, dst)
		}
	}

	for _, m := range generatedmodules {
		fset := token.NewFileSet()
		o, err := parser.ParseFile(fset, m.fname, m.original, 0)
		if err != nil {
			return roots, err
		}

		mfn, err := parser.ParseFile(token.NewFileSet(), m.fname, m.main.String(), 0)
		if err != nil {
			return roots, err
		}

		main := astcodec.FindFunctionDecl(&packages.Package{Syntax: []*ast.File{mfn}}, astcodec.FindFunctionsByName("main"))
		main.Type.Func = token.NoPos
		main.Type.Params.Opening = token.NoPos

		result := astcodec.ReplaceFunction(o, main, astcodec.FindFunctionsByName("main"))
		if err = rewrite(fset.File(result.Pos()), m.fname, result); err != nil {
			return roots, err
		}

		roots = append(roots, m.fname)
	}

	return roots, nil
}
