package compile

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/execx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/tracex"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"golang.org/x/mod/modfile"
)

func InitGolang(ctx context.Context, dir string, packages ...string) error {
	cmd := exec.CommandContext(ctx, "go", "mod", "init", "eg/compute")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errorsx.Wrapf(err, "unable to initialize go.mod: %s", cmd.Dir)
	}

	return InitPackages(ctx, dir, "-u", packages...)
}

// InitGolangModule initializes a new module like InitGolang and registers it
// with a workspace so the go command keeps resolving it alongside its siblings.
// An existing workspace is modified and the module's go version is aligned with
// the workspace's, otherwise every following go command refuses to run. When
// there is no workspace one is created, see workspacePath.
func InitGolangModule(ctx context.Context, dir string, packages ...string) error {
	cmd := exec.CommandContext(ctx, "go", "mod", "init", "eg/compute")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	if err := execx.MaybeRun(cmd); err != nil {
		return errorsx.Wrapf(err, "unable to initialize go.mod: %s", cmd.Dir)
	}

	path, exists, err := workspacePath(dir)
	if err != nil {
		return err
	}

	if path != "" {
		if err = useWorkspace(ctx, dir, path, exists); err != nil {
			return err
		}
	}

	return InitPackages(ctx, dir, "-u", packages...)
}

// useWorkspace adds the module in dir to the workspace at path, creating the
// workspace when it does not exist. a created workspace takes the go version of
// the toolchain, which is the version go mod init gave the module.
func useWorkspace(ctx context.Context, dir, path string, exists bool) error {
	root := filepath.Dir(path)

	if !exists {
		cmd := exec.CommandContext(ctx, "go", "work", "init")
		cmd.Dir = root
		cmd.Stderr = os.Stderr
		if err := execx.MaybeRun(cmd); err != nil {
			return errorsx.Wrapf(err, "unable to create the workspace: %s", path)
		}
	} else {
		raw, err := os.ReadFile(path)
		if err != nil {
			return errorsx.Wrapf(err, "unable to read workspace: %s", path)
		}

		work, err := modfile.ParseWork(path, raw, nil)
		if err != nil {
			return errorsx.Wrapf(err, "unable to parse workspace: %s", path)
		}

		if work.Go != nil {
			cmd := exec.CommandContext(ctx, "go", "mod", "edit", "-go="+work.Go.Version)
			cmd.Dir = dir
			cmd.Stderr = os.Stderr
			if err := execx.MaybeRun(cmd); err != nil {
				return errorsx.Wrapf(err, "unable to align go.mod with the workspace go version: %s", work.Go.Version)
			}
		}
	}

	rel, err := workspaceRel(root, dir)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "go", "work", "use", rel)
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	if err := execx.MaybeRun(cmd); err != nil {
		return errorsx.Wrapf(err, "unable to add the module to the workspace: %s", path)
	}

	return nil
}

// workspaceRel returns the use directive for the module in dir relative to the
// workspace root, e.g. ./.eg
func workspaceRel(root, dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", errorsx.Wrapf(err, "unable to resolve directory: %s", dir)
	}

	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", errorsx.Wrapf(err, "unable to locate the module within the workspace: %s", root)
	}

	if !strings.HasPrefix(rel, "..") {
		rel = "." + string(filepath.Separator) + rel
	}

	return rel, nil
}

// DropWorkspaceModule removes the module in dir from the use directives of the
// workspace that governs it. it does nothing when there is no existing workspace.
func DropWorkspaceModule(ctx context.Context, dir string) error {
	path, exists, err := workspacePath(dir)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	root := filepath.Dir(path)
	rel, err := workspaceRel(root, dir)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "go", "work", "edit", "-dropuse", rel)
	cmd.Dir = root
	cmd.Stderr = os.Stderr
	if err := execx.MaybeRun(cmd); err != nil {
		return errorsx.Wrapf(err, "unable to remove the module from the workspace: %s", path)
	}

	return nil
}

// workspacePath returns the go.work to register the module in dir with, and
// whether that file already exists. returns an empty path when workspaces are
// disabled with GOWORK=off. in order:
//   - GOWORK when set, or the nearest existing go.work in dir or its parents.
//   - a new go.work at the nearest parent containing a .git.
//   - a new go.work in the current working directory.
func workspacePath(dir string) (string, bool, error) {
	path := os.Getenv("GOWORK")
	if path == "off" {
		return "", false, nil
	}

	if path != "" {
		return path, true, nil
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false, errorsx.Wrapf(err, "unable to resolve directory: %s", dir)
	}

	// LocateWithin returns the bare name when nothing is found.
	if found := fsx.LocateWithin(abs, "go.work"); found != "go.work" {
		return found, true, nil
	}

	if found := fsx.LocateWithin(abs, ".git"); found != ".git" {
		return filepath.Join(filepath.Dir(found), "go.work"), false, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", false, errorsx.Wrapf(err, "unable to determine the current directory")
	}

	return filepath.Join(cwd, "go.work"), false, nil
}

func InitPackages(ctx context.Context, dir string, update string, packages ...string) error {
	for _, pkg := range packages {
		cmd := exec.CommandContext(ctx, "go", "get", "-modcacherw", update, pkg)
		cmd.Dir = dir
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return errorsx.Wrapf(err, "unable to download default packages: %s", cmd.String())
		}
	}

	return nil
}

func InitGolangTidy(ctx context.Context, dir string) error {
	cmd := exec.CommandContext(ctx, "go", "mod", "tidy", "-modcacherw")
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errorsx.Wrapf(err, "unable to tidy go.mod: %s", cmd.Dir)
	}

	return nil
}

func EnsureRequiredPackages(ctx context.Context, dir string, packages ...string) error {
	defaultPackages := []string{
		"get",
		"-modcacherw",
		"google.golang.org/genproto@latest",
		"github.com/egdaemon/eg/runtime/autowasinet",
		"github.com/egdaemon/eg/interp/events",
	}

	cmd := exec.CommandContext(ctx, "go", append(defaultPackages, packages...)...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return errorsx.Wrapf(err, "unable to download default packages: %s - %s", cmd.Dir, cmd.String())
	}

	return nil
}

func FromTranspiled(ctx context.Context, ws workspaces.Context, m ...transpile.Compiled) (modules []transpile.Compiled, err error) {
	modules = make([]transpile.Compiled, 0, len(m))

	for _, root := range m {
		var (
			path string
		)

		if path, err = filepath.Rel(ws.TransDir, root.Path); err != nil {
			return modules, errorsx.Wrapf(err, "base(%s) %s", ws.TransDir, root.Path)
		}

		path = workspaces.TrimRoot(path, filepath.Base(ws.GenModDir))
		path = workspaces.ReplaceExt(path, ".wasm")
		path = filepath.Join(ws.Root, ws.BuildDir, path)

		if !root.Generated {
			modules = append(modules, transpile.Compiled{Path: path, Generated: root.Generated})
		}

		if _, err = os.Stat(path); err == nil {
			// nothing to do.
			continue
		}

		mpath := strings.TrimPrefix(strings.TrimPrefix(root.Path, ws.TransDir), "/")

		// fsx.PrintDir(os.DirFS(filepath.Join(ws.Root, ws.TransDir)))

		tracex.Println("compiling module", root.Path, mpath)
		if err = Run(ctx, filepath.Join(ws.Root, ws.TransDir), mpath, path); err != nil {
			return modules, err
		}
	}

	return modules, errorsx.Wrap(err, "compilation failed")
}

func Run(ctx context.Context, dir, module string, output string) (err error) {
	debugx.Println("compiling initiated", dir, module, "->", output)
	defer debugx.Println("compiling completed", dir, module, "->", output)

	cmd := exec.CommandContext(ctx, "go", "build", "-mod=readonly", "-modcacherw", "-trimpath", "-o", output, strings.TrimPrefix(module, dir+"/"))
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	debugx.Println("executing", dir, cmd.String())

	return cmd.Run()
}
