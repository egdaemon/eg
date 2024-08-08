package compute

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/wasix"
	pc8s "github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid"
	"google.golang.org/grpc"
)

type c8sLocal struct {
	cmdopts.RuntimeResources
	Dir           string   `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	Containerfile string   `arg:"" help:"path to the container file to run" default:"Containerfile"`
	SSHKeyPath    string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Environment   []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	GitRemote     string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	Endpoint      string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/manager/" hidden:"true"`
}

func (t c8sLocal) Run(gctx *cmdopts.Global) (err error) {
	var (
		tmpdir     string
		ws         workspaces.Context
		cc         grpc.ClientConnInterface
		environio  *os.File
		uid                            = uuid.Must(uuid.NewV7())
		sshmount   runners.AgentOption = runners.AgentOptionNoop
		sshenvvar  runners.AgentOption = runners.AgentOptionNoop
		envvar     runners.AgentOption = runners.AgentOptionNoop
		mounthome  runners.AgentOption = runners.AgentOptionNoop
		mountegbin runners.AgentOption = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
	)

	if tmpdir, err = os.MkdirTemp("", "eg.c8s.upload.*"); err != nil {
		return errorsx.Wrap(err, "unable to  create temporary directory")
	}

	defer func() {
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	if ws, err = workspaces.New(gctx.Context, tmpdir, ".eg", ""); err != nil {
		return err
	}

	egdir := filepath.Join(ws.Root, ws.ModuleDir)
	autoruncontainer := filepath.Join(ws.Root, ws.WorkingDir, "workspace", "Containerfile")
	if err = fsx.MkDirs(0700, filepath.Dir(autoruncontainer)); err != nil {
		return err
	}
	fsx.PrintFS(os.DirFS(ws.Root))

	if err = fsx.CloneTree(gctx.Context, egdir, ".bootstrap.c8s", embeddedc8supload); err != nil {
		return err
	}

	if err = compile.InitGolang(gctx.Context, egdir, cmdopts.ModPath()); err != nil {
		return err
	}

	if err = iox.Copy(t.Containerfile, autoruncontainer); err != nil {
		return err
	}

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(gctx.Context)
	if err != nil {
		return err
	}

	modules, err := compile.FromTranspiled(gctx.Context, ws, roots...)
	if err != nil {
		return errorsx.Wrap(err, "unable to transpile")
	}

	if len(modules) == 0 {
		return errors.New("no usable modules detected")
	}

	log.Println("modules", modules)

	environpath := filepath.Join(ws.Root, ws.RuntimeDir, "environ.env")
	if environio, err = os.Create(environpath); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

	if cc, err = daemons.AutoRunnerClient(
		gctx,
		ws,
		uid.String(),
		mounthome,
		mountegbin,
		envvar,
		sshmount,
		sshenvvar,
		runners.AgentOptionEnviron(environpath),
		runners.AgentOptionMounts(runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.CacheDir), "/cache")),
	); err != nil {
		return errorsx.Wrap(err, "unable to setup runner")
	}

	if err = wasix.WarmCacheDirectory(gctx.Context, filepath.Join(ws.Root, ws.BuildDir), wasix.WazCacheDir(filepath.Join(ws.Root, ws.RuntimeDir))); err != nil {
		log.Println("unable to prewarm wasi directory cache", err)
	}

	runner := pc8s.NewProxyClient(cc)

	for _, m := range modules {
		options := []string{}

		options = append(options,
			"--volume", runners.AgentMountReadOnly(m.Path, "/opt/egmodule.wasm"),
			"--cpus", strconv.FormatUint(t.RuntimeResources.Cores, 10),
		)

		_, err = runner.Module(gctx.Context, &pc8s.ModuleRequest{
			Image:   "eg",
			Name:    fmt.Sprintf("eg-%s", uid.String()),
			Mdir:    ws.BuildDir, // TODO REVISIT THIS.
			Options: options,
		})

		if err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	return nil
}
