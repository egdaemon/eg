package egautogen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"

	"github.com/egdaemon/eg/internal/errorsx"
)

const (
	// maximum number of call sites rendered, in addition to the tests calling the function.
	maxCallerExamples = 3

	// call sites longer than this are cut short.
	maxCallerLines = 20
)

// callindex records, for each file of a module, the functions it calls so the files calling a
// function can be found without parsing the module again. built once per module root.
//
// calls are keyed precisely so unrelated functions with the same name don't match:
// "dir:<absolute dir>.<Name>" for calls to a function of the same package, and
// "<import path>.<Name>" for calls qualified by an import.
type callindex struct {
	modpath string
	root    string
	files   map[string][]string // key -> files containing a call.
}

type callcache struct {
	mu    sync.Mutex
	built map[string]*callindex
}

// index is the call index of the module at root, built the first time it's needed.
func (t *callcache) index(root string) (*callindex, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if idx, ok := t.built[root]; ok {
		return idx, nil
	}

	idx, err := buildCallIndex(root)
	if err != nil {
		return nil, err
	}

	t.built[root] = idx
	return idx, nil
}

// callers renders up to maxCallerExamples of the statements calling the function name declared
// in dir, from within the function's module. calls from the same package come first.
func (t *callcache) callers(dir string, name string) ([]string, error) {
	// methods are not supported, the receiver can't be resolved without type information.
	if strings.Contains(name, ".") {
		return nil, nil
	}

	absdir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	root, err := findModuleRoot(absdir)
	if err != nil {
		return nil, err
	}

	idx, err := t.index(root)
	if err != nil {
		return nil, err
	}

	rel, err := filepath.Rel(root, absdir)
	if err != nil {
		return nil, err
	}

	importpath := path.Join(idx.modpath, filepath.ToSlash(rel))
	files := append(
		append([]string(nil), idx.files["dir:"+absdir+"."+name]...),
		idx.files[importpath+"."+name]...,
	)

	var examples []string
	for _, file := range files {
		rendered, err := renderCalls(file, absdir, importpath, name)
		if err != nil {
			return nil, err
		}

		for _, r := range rendered {
			examples = append(examples, strings.Replace(r, root+string(filepath.Separator), "", 1))
			if len(examples) >= maxCallerExamples {
				return examples, nil
			}
		}
	}

	return examples, nil
}

func buildCallIndex(root string) (*callindex, error) {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to read go.mod")
	}

	idx := &callindex{modpath: modfile.ModulePath(raw), root: root, files: map[string][]string{}}
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if p != root && skipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(p) != ".go" {
			return nil
		}

		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil // unparsable files can't be examples.
		}

		seen := map[string]bool{}
		imports := fileImports(f)
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if key := callKey(call, filepath.Dir(p), imports); key != "" && !seen[key] {
					seen[key] = true
					idx.files[key] = append(idx.files[key], p)
				}
			}
			return true
		})

		return nil
	})

	for _, files := range idx.files {
		sort.Strings(files)
	}

	return idx, errorsx.Wrap(err, "unable to index calls")
}

func skipDir(name string) bool {
	return name == "vendor" || name == "testdata" || name == "node_modules" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// fileImports maps the name each import is referred to by within the file to its import path.
func fileImports(f *ast.File) map[string]string {
	imports := make(map[string]string, len(f.Imports))
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		name := path.Base(p)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = p
	}

	return imports
}

// callKey is the index key of the function the call invokes, empty if it isn't a plain function call.
func callKey(call *ast.CallExpr, dir string, imports map[string]string) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return "dir:" + dir + "." + fn.Name
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok {
			if p, ok := imports[x.Name]; ok {
				return p + "." + fn.Sel.Name
			}
		}
	}

	return ""
}

// renderCalls renders the statement of each call in file to the function, prefixed by where it is from.
func renderCalls(file string, dir string, importpath string, name string) ([]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, errorsx.Wrapf(err, "unable to parse: %s", file)
	}

	var (
		rendered []string
		stack    []ast.Node
		imports  = fileImports(f)
		target   = importpath + "." + name
		local    = "dir:" + dir + "." + name
	)

	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if key := callKey(call, filepath.Dir(file), imports); key != target && key != local {
			return true
		}

		if node := enclosingStatement(stack); node != nil {
			if s, err := renderNode(fset, node); err == nil {
				rendered = append(rendered, "// "+file+"\n"+truncateLines(s, maxCallerLines))
			}
		}

		return true
	})

	return rendered, nil
}

// enclosingStatement is the innermost statement, or top level declaration, containing the last node of the stack.
func enclosingStatement(stack []ast.Node) ast.Node {
	for i := len(stack) - 2; i >= 0; i-- {
		switch n := stack[i].(type) {
		case ast.Stmt:
			if _, block := n.(*ast.BlockStmt); !block {
				return n
			}
		case *ast.GenDecl:
			return n
		}
	}

	return nil
}

func truncateLines(s string, max int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= max {
		return s
	}

	return strings.Join(lines[:max], "\n") + "\n// ..."
}
