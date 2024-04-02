package transpile

import (
	"bytes"
	"context"
	"errors"
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

	"github.com/davecgh/go-spew/spew"

	"github.com/dave/jennifer/jen"
	"github.com/egdaemon/eg/astbuild"
	"github.com/egdaemon/eg/astcodec"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/workspaces"

	"golang.org/x/tools/go/packages"
)

type Context struct {
	Workspace workspaces.Context
}

type Compiled struct {
	Path      string
	Generated bool
}

type Transpiler interface {
	Run(ctx context.Context) (roots []Compiled, err error)
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

func (t golang) Run(ctx context.Context) (roots []Compiled, err error) {
	type module struct {
		fname    string
		original *bytes.Buffer
		main     *bytes.Buffer
		pos      token.Position
	}

	var (
		dst string
		pkg *packages.Package
	)

	pkgc := astcodec.DefaultPkgLoad(
		astcodec.LoadDir(filepath.Join(t.Context.Workspace.Root, t.Context.Workspace.ModuleDir, t.Context.Workspace.Module)),
		astcodec.AutoFileSet,
	)

	if pkg, err = astcodec.Load(pkgc); err != nil {
		return roots, errorsx.Wrap(err, "unable to load package")
	}
	generatedmodules := make([]*module, 0, 128)

	transform := func(ftoken *token.File, gendir string, c *ast.File) error {
		var (
			yakident = "yak"
		)

		if imp := astcodec.FindImport(c, astcodec.FindImportsByPath("github.com/egdaemon/eg/runtime/wasi/yak")); imp != nil && imp.Name != nil {
			yakident = imp.Name.String()
		}

		refexpr := astbuild.SelExpr(yakident, "Ref")
		refmodule := astbuild.SelExpr(yakident, "Module")
		refexec := astbuild.SelExpr(yakident, "Exec")

		// make a clone of the buffer
		buf := bytes.NewBuffer(nil)
		if err = printer.Fprint(buf, pkg.Fset, c); err != nil {
			return err
		}

		moduleExpr := func(ce *ast.CallExpr) bool {
			return astcodec.TypePattern(refmodule)(ce.Fun)
		}

		execExpr := func(ce *ast.CallExpr) bool {
			return astcodec.TypePattern(refexec)(ce.Fun)
		}

		generr := error(nil)
		genmod := astcodec.NewCallExprReplacement(func(ce *ast.CallExpr) *ast.CallExpr {
			if len(ce.Args) < 3 {
				log.Println("unable to transpile module call")
				return ce
			}

			ctxarg := ce.Args[0]
			rarg := ce.Args[1]
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

			mainbuf := bytes.NewBuffer(nil)
			if cause := main.Render(mainbuf); cause != nil {
				generr = errors.Join(generr, cause)
			}

			genwasm := filepath.Join(gendir, fmt.Sprintf("module.%d.%d.wasm", pos.Line, pos.Column))
			m := &module{
				fname:    filepath.Join(t.Workspace.GenModDir, gendir, fmt.Sprintf("module.%d.%d.go", pos.Line, pos.Column)),
				original: buf,
				main:     mainbuf,
				pos:      pos,
			}
			generatedmodules = append(generatedmodules, m)

			return astbuild.CallExpr(astbuild.SelExpr(yakident, "UnsafeRunner"), ctxarg, astbuild.CallExpr(astbuild.SelExpr(types.ExprString(rarg), "ToModuleRunner")), astbuild.StringLiteral(genwasm))
		}, moduleExpr)

		genexec := astcodec.NewCallExprReplacement(func(ce *ast.CallExpr) *ast.CallExpr {
			if len(ce.Args) < 2 {
				log.Println("unable to transpile exec call", len(ce.Args))
				return ce
			}

			ctxarg := ce.Args[0]
			rarg := ce.Args[1]

			pos := pkg.Fset.PositionFor(ce.Pos(), true)
			pos.Filename = strings.TrimPrefix(pos.Filename, t.Workspace.Root+"/")
			genwasm := filepath.Join(gendir, fmt.Sprintf("module.%d.%d.wasm", pos.Line, pos.Column))

			return astbuild.CallExpr(astbuild.SelExpr(yakident, "UnsafeRunner"), ctxarg, rarg, astbuild.StringLiteral(genwasm))
		}, execExpr)

		v := astcodec.Multivisit(
			// astcodec.Filter(grapher(pkg.Fset), moduleExpr),
			replaceRef(yakident, refexpr),
			genmod,
			genexec,
		)

		ast.Walk(v, c)

		return generr
	}

	rewrite := func(ftoken *token.File, dst string, c ast.Node) error {
		var (
			iodst     *os.File
			formatted string
			buf       = bytes.NewBuffer(nil)
		)

		if err = (&printer.Config{Mode: printer.TabIndent}).Fprint(buf, pkg.Fset, c); err != nil {
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

		if gendir, err = filepath.Rel(filepath.Join(t.Context.Workspace.Root, t.Context.Workspace.ModuleDir), workspaces.ReplaceExt(ftoken.Name(), ".wasm.d")); err != nil {
			return roots, err
		}

		if err = transform(ftoken, gendir, c); err != nil {
			return roots, err
		}

		log.Println("writing transformed to", dst)
		if err = rewrite(ftoken, dst, c); err != nil {
			return roots, err
		}

		if mainfn := astcodec.FindFunctionDecl(pkg, astcodec.FindFunctionsByName("main")); mainfn != nil {
			roots = append(roots, Compiled{Path: dst})
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
		log.Println("workspace", spew.Sdump(t.Context.Workspace))
		log.Println("root", t.Context.Workspace.Root)
		log.Println("original", m.fname)
		// dst, err := workspaces.PathTranspiled(t.Context.Workspace, filepath.Join(t.Context.Workspace.Root, t.Context.Workspace.ModuleDir), filepath.Join(t.Context.Workspace.Root, m.fname))
		// if err != nil {
		// 	return roots, errorsx.Wrap(err, "unable to generate dst for generated module")
		// }

		// log.Println("writing transformed to", dst)
		// if err = rewrite(fset.File(result.Pos()), dst, result); err != nil {
		if err = rewrite(fset.File(result.Pos()), m.fname, result); err != nil {
			return roots, err
		}

		roots = append(roots, Compiled{Path: m.fname, Generated: true})
	}

	return roots, nil
}
