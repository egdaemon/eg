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

	"github.com/dave/jennifer/jen"
	"github.com/egdaemon/eg/astbuild"
	"github.com/egdaemon/eg/astcodec"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/langx"
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

type module struct {
	fname    string
	original *bytes.Buffer
	main     *bytes.Buffer
	pos      token.Position
}

type golang struct {
	Context
}

func (t golang) Run(ctx context.Context) (roots []Compiled, err error) {
	var (
		dst  string
		pset []*packages.Package
	)
	transdir := filepath.Join(t.Context.Workspace.Root, t.Context.Workspace.TransDir)

	err = fsx.CloneTree(ctx, transdir, ".", os.DirFS(filepath.Join(t.Context.Workspace.Root, t.Context.Workspace.ModuleDir)))
	if err != nil {
		return roots, err
	}

	pkgc := astcodec.DefaultPkgLoad(
		astcodec.LoadDir(transdir),
		astcodec.AutoFileSet,
		astcodec.DisableGowork, // dont want to do this but until I figure out the issue.
	)

	generatedmodules := make([]*module, 0, 128)
	if pset, err = packages.Load(pkgc, "./..."); err != nil {
		return nil, err
	}

	for _, pkg := range pset {
		target := filepath.Join(langx.Autoderef(pkg.Module).Path, t.Workspace.Module)
		for _, c := range pkg.Syntax {
			var (
				gendir string
				genm   []*module
			)
			ftoken := pkg.Fset.File(c.Pos())

			if dst, err = workspaces.PathTranspiled(t.Context.Workspace, transdir, ftoken.Name()); err != nil {
				return roots, err
			}

			if gendir, err = filepath.Rel(transdir, workspaces.ReplaceExt(ftoken.Name(), ".wasm.d")); err != nil {
				return roots, err
			}

			if genm, err = transform(t.Workspace, pkg.Fset, gendir, c); err != nil {
				return roots, err
			}

			if err = rewrite(filepath.Join(t.Context.Workspace.Root, dst), pkg.Fset, c); err != nil {
				return roots, err
			}

			if target != pkg.ID {
				debugx.Println("ignoring", target, pkg.ID)
				continue
			}

			generatedmodules = append(generatedmodules, genm...)

			if mainfn := astcodec.FindFunctionDecl(pkg, astcodec.FindFunctionsByName("main")); mainfn != nil {
				roots = append(roots, Compiled{Path: dst})
			}
		}
	}

	for _, m := range generatedmodules {
		o, err := parser.ParseFile(pkgc.Fset, m.fname, m.original, 0)
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
		log.Println("original", m.fname)

		if err = rewrite(filepath.Join(t.Context.Workspace.Root, m.fname), token.NewFileSet(), result); err != nil {
			return roots, err
		}

		roots = append(roots, Compiled{Path: m.fname, Generated: true})
	}

	return roots, nil
}

func transform(ws workspaces.Context, fset *token.FileSet, gendir string, c *ast.File) (generatedmodules []*module, err error) {
	var (
		egident    = "eg"
		egenvident = "egenv"
	)

	if imp := astcodec.FindImport(c, astcodec.FindImportsByPath("github.com/egdaemon/eg/runtime/wasi/eg")); imp != nil && imp.Name != nil {
		egident = imp.Name.String()
	}

	if imp := astcodec.FindImport(c, astcodec.FindImportsByPath("github.com/egdaemon/eg/runtime/wasi/egenv")); imp != nil && imp.Name != nil {
		egenvident = imp.Name.String()
	}

	refexpr := astbuild.SelExpr(egident, "Ref")
	refmodule := astbuild.SelExpr(egident, "Module")
	refexec := astbuild.SelExpr(egident, "Exec")

	// make a clone of the buffer
	buf := bytes.NewBuffer(nil)
	if err = printer.Fprint(buf, fset, c); err != nil {
		return generatedmodules, err
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

		pos := fset.PositionFor(ce.Pos(), true)
		pos.Filename = strings.TrimPrefix(pos.Filename, ws.Root+"/")

		main := jen.NewFile("main")
		main.Commentf("automatically generated from: %s", pos)
		main.Func().Id("main").Params().Block(
			jen.List(jen.Id("ctx"), jen.Id("done")).Op(":=").Add(
				jen.Qual("context", "WithTimeout").Call(
					jen.Qual("context", "Background").Call(),
					jen.Qual(egenvident, "TTL").Call(),
				),
			),
			jen.Defer().Add(jen.Id("done").Call()),
			jen.If(
				jen.Id("err").Op(":=").Add(jen.Qual(egident, "Perform").Call(
					jen.Id("ctx"),
					jen.Qual(egident, "Sequential").Call(statements...)),
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
			fname:    filepath.Join(ws.GenModDir, gendir, fmt.Sprintf("module.%d.%d.go", pos.Line, pos.Column)),
			original: buf,
			main:     mainbuf,
			pos:      pos,
		}
		generatedmodules = append(generatedmodules, m)
		return astbuild.CallExpr(astbuild.SelExpr(egident, "UnsafeRunner"), ctxarg, astbuild.CallExpr(astbuild.SelExpr(types.ExprString(rarg), "ToModuleRunner")), astbuild.StringLiteral(genwasm))
	}, moduleExpr)

	genexec := astcodec.NewCallExprReplacement(func(ce *ast.CallExpr) *ast.CallExpr {
		if len(ce.Args) < 2 {
			log.Println("unable to transpile exec call", len(ce.Args))
			return ce
		}

		ctxarg := ce.Args[0]
		rarg := ce.Args[1]

		pos := fset.PositionFor(ce.Pos(), true)
		pos.Filename = strings.TrimPrefix(pos.Filename, ws.Root+"/")
		genwasm := filepath.Join(gendir, fmt.Sprintf("module.%d.%d.wasm", pos.Line, pos.Column))

		return astbuild.CallExpr(astbuild.SelExpr(egident, "UnsafeExec"), ctxarg, rarg, astbuild.StringLiteral(genwasm))
	}, execExpr)

	v := astcodec.Multivisit(
		replaceRef(egident, refexpr),
		genmod,
		genexec,
	)
	ast.Walk(v, c)

	return generatedmodules, generr
}

func rewrite(dst string, fset *token.FileSet, c ast.Node) (err error) {
	var (
		iodst     *os.File
		formatted string
		buf       = bytes.NewBuffer(nil)
	)

	debugx.Println("writing transformed to", dst)
	if err = (&printer.Config{}).Fprint(buf, fset, c); err != nil {
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
