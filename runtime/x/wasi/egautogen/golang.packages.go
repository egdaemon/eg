package egautogen

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/x/wasi/eggolang"
)

// packageBlock labels the rendered api of a package with its name and import path.
func packageBlock(name string, importpath string, body string) string {
	if importpath == "" {
		return fmt.Sprintf("package %s\n%s", name, body)
	}

	return fmt.Sprintf("package %s // import %q\n%s", name, importpath, body)
}

// collectPackages renders the exported function signatures of each of the packages.
func collectPackages(fromdir string, importpaths []string) (string, error) {
	rendered := make([]string, 0, len(importpaths))
	for _, importpath := range importpaths {
		dir, err := resolvePackage(fromdir, importpath)
		if err != nil {
			return "", err
		}

		signatures, err := packageSignatures(dir)
		if err != nil {
			return "", err
		}

		rendered = append(rendered, packageBlock(filepath.Base(importpath), importpath, signatures))
	}

	return strings.Join(rendered, "\n\n"), nil
}

// resolvePackage finds the directory of the package, looking in the standard library, the module
// containing fromdir, then the module's vendor directory, then the module cache.
func resolvePackage(fromdir string, importpath string) (string, error) {
	if first, _, _ := strings.Cut(importpath, "/"); !strings.Contains(first, ".") {
		if dir := filepath.Join(envx.String(runtime.GOROOT(), "GOROOT"), "src", filepath.FromSlash(importpath)); isDir(dir) {
			return dir, nil
		}
	}

	root, err := findModuleRoot(fromdir)
	if err != nil {
		return "", err
	}

	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", errorsx.Wrap(err, "unable to read go.mod")
	}

	mod, err := modfile.ParseLax(filepath.Join(root, "go.mod"), raw, nil)
	if err != nil {
		return "", errorsx.Wrap(err, "unable to parse go.mod")
	}

	if mod.Module != nil {
		if rel, ok := trimModule(mod.Module.Mod.Path, importpath); ok {
			return existingDir(filepath.Join(root, rel), importpath)
		}
	}

	if dir := filepath.Join(root, "vendor", filepath.FromSlash(importpath)); isDir(dir) {
		return dir, nil
	}

	cache := envx.String(eggolang.CacheModuleDirectory(), "GOMODCACHE")
	for _, req := range mod.Require {
		rel, ok := trimModule(req.Mod.Path, importpath)
		if !ok {
			continue
		}

		escaped, err := module.EscapePath(req.Mod.Path)
		if err != nil {
			return "", errorsx.Wrapf(err, "unable to escape module path: %s", req.Mod.Path)
		}

		return existingDir(filepath.Join(cache, escaped+"@"+req.Mod.Version, rel), importpath)
	}

	return "", fmt.Errorf("unable to locate package: %s", importpath)
}

// trimModule returns the path of importpath relative to the module, if it is within it.
func trimModule(modpath string, importpath string) (string, bool) {
	if importpath == modpath {
		return "", true
	}

	return strings.CutPrefix(importpath, modpath+"/")
}

func existingDir(dir string, importpath string) (string, error) {
	if !isDir(dir) {
		return "", fmt.Errorf("unable to locate package: %s (%s)", importpath, dir)
	}

	return dir, nil
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func findModuleRoot(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	for d := abs; ; d = filepath.Dir(d) {
		if _, err := os.Stat(filepath.Join(d, "go.mod")); err == nil {
			return d, nil
		}

		if d == filepath.Dir(d) {
			return "", fmt.Errorf("no go.mod found for: %s", dir)
		}
	}
}

// packageSignatures lists the exported types of the package in dir, each declared in full but for its unexported fields and methods, followed by its exported
// functions, one signature per line. types and functions are sorted by name.
// methods and test files are skipped, as are the printf style variants of a function (Equalf for Equal).
func packageSignatures(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", errorsx.Wrapf(err, "unable to read package: %s", dir)
	}

	fset := token.NewFileSet()
	decls := map[string]*ast.FuncDecl{}
	specs := map[string]*ast.TypeSpec{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".go" || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}

		f, err := parser.ParseFile(fset, filepath.Join(dir, e.Name()), nil, parser.SkipObjectResolution)
		if err != nil {
			return "", errorsx.Wrapf(err, "unable to parse: %s", filepath.Join(dir, e.Name()))
		}

		// only what a caller can use, like go doc: unexported declarations, fields and methods are removed.
		ast.FileExports(f)

		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					decls[d.Name.Name] = d
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.IsExported() {
						specs[ts.Name.Name] = ts
					}
				}
			}
		}
	}

	names := make([]string, 0, len(decls))
	for name := range decls {
		if base, ok := strings.CutSuffix(name, "f"); ok && decls[base] != nil {
			continue
		}

		names = append(names, name)
	}
	sort.Strings(names)

	lines := make([]string, 0, len(names))
	for _, name := range names {
		sig := *decls[name]
		sig.Doc, sig.Body = nil, nil

		rendered, err := renderNode(fset, &sig)
		if err != nil {
			return "", errorsx.Wrapf(err, "unable to render: %s", name)
		}

		lines = append(lines, rendered)
	}

	typenames := make([]string, 0, len(specs))
	for name := range specs {
		typenames = append(typenames, name)
	}
	sort.Strings(typenames)

	types := make([]string, 0, len(typenames))
	for _, name := range typenames {
		spec := *specs[name]
		spec.Doc, spec.Comment = nil, nil

		rendered, err := renderNode(fset, &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&spec}})
		if err != nil {
			return "", errorsx.Wrapf(err, "unable to render: %s", name)
		}

		types = append(types, rendered)
	}

	return strings.Join(nonEmpty(strings.Join(types, "\n\n"), strings.Join(lines, "\n")), "\n\n"), nil
}

// importsOf lists the import paths of the templates, the file containing decl, and the existing test file.
// sources that don't parse are skipped, templates are commonly a bare function without a package clause.
func importsOf(pkg *gopkg, decl *ast.FuncDecl, existing []byte, templates []string) (imports []string) {
	collect := func(f *ast.File) {
		for _, spec := range f.Imports {
			if v, err := strconv.Unquote(spec.Path.Value); err == nil {
				imports = append(imports, v)
			}
		}
	}

	for _, src := range append([]string{string(existing)}, templates...) {
		if !strings.HasPrefix(strings.TrimSpace(src), "package ") {
			src = "package p\n" + src
		}

		if f, err := parser.ParseFile(token.NewFileSet(), "", src, parser.ImportsOnly|parser.SkipObjectResolution); err == nil {
			collect(f)
		}
	}

	for _, f := range pkg.files {
		if decl.Pos() >= f.Pos() && decl.End() <= f.End() {
			collect(f)
		}
	}

	return imports
}
