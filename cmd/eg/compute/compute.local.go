package compute

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/wasix"
	pc8s "github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigraph"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
	"github.com/gofrs/uuid"
	"google.golang.org/grpc"
)

type local struct {
	Dir           string   `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir     string   `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
	Name          string   `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:""`
	Privileged    bool     `name:"privileged" help:"run the initial container in privileged mode"`
	Dirty         bool     `name:"dirty" help:"include user directories and environment variables" hidden:"true"`
	MountEnvirons string   `name:"environ" help:"environment file to pass to the module" default:""`
	EnvVars       []string `name:"env" short:"e" help:"environment variables to import" default:""`
	SSHAgent      bool     `name:"sshagent" help:"enable ssh agent" hidden:"true"`
	GitRemote     string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference  string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_default_reference}"`
}

func (t local) Run(gctx *cmdopts.Global) (err error) {
	var (
		ws         workspaces.Context
		repo       *git.Repository
		uid        = uuid.Must(uuid.NewV7())
		ebuf       = make(chan *ffigraph.EventInfo)
		environio  *os.File
		cc         grpc.ClientConnInterface
		sshmount   runners.AgentOption = runners.AgentOptionNoop
		sshenvvar  runners.AgentOption = runners.AgentOptionNoop
		envvar     runners.AgentOption = runners.AgentOptionNoop
		mounthome  runners.AgentOption = runners.AgentOptionNoop
		mountegbin runners.AgentOption = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
	)

	if ws, err = workspaces.New(gctx.Context, t.Dir, t.ModuleDir, t.Name); err != nil {
		return errorsx.Wrap(err, "unable to setup workspace")
	}

	defer os.RemoveAll(filepath.Join(ws.Root, ws.RuntimeDir))

	if err = os.Remove(filepath.Join(ws.Root, ws.WorkingDir)); err != nil {
		return errorsx.Wrap(err, "unable to symlink working directory")
	}

	if err = os.Symlink(ws.Root, filepath.Join(ws.Root, ws.WorkingDir)); err != nil {
		return errorsx.Wrap(err, "unable to symlink working directory")
	}

	environpath := filepath.Join(ws.Root, ws.RuntimeDir, "environ.env")
	if environio, err = os.Create(environpath); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

	if repo, err = git.PlainOpen(ws.Root); err != nil {
		return errorsx.Wrap(err, "unable to open git repository")
	}

	envb := envx.Build().
		FromPath(t.MountEnvirons).
		FromEnv(t.EnvVars...).
		FromEnv(os.Environ()...).
		FromEnviron(errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))...).
		Var("EG_INTERNAL_GIT_CLONE_ENABLED", strconv.FormatBool(false)) // hack to disable cloning

	if t.Dirty {
		mounthome = runners.AgentOptionAutoMountHome(errorsx.Must(os.UserHomeDir()))
	}

	if t.SSHAgent {
		sshmount = runners.AgentOptionMounts(
			runners.AgentMountOverlay(
				filepath.Join(errorsx.Must(os.UserHomeDir()), ".ssh"),
				"/root/.ssh",
			),
			runners.AgentMountReadWrite(
				envx.String("", "SSH_AUTH_SOCK"),
				"/opt/egruntime/ssh.agent.socket",
			),
		)

		sshenvvar = runners.AgentOptionEnvKeys("SSH_AUTH_SOCK=/opt/egruntime/ssh.agent.socket")
		envb.FromEnviron("SSH_AUTH_SOCK=/opt/egruntime/ssh.agent.socket")
	}

	// envx.Debug(errorsx.Zero(envb.Environ())...)

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

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

	// enable event logging
	// w, err := events.NewAgentClient(cc).Watch(ctx.Context, &events.RunWatchRequest{Run: &events.RunMetadata{Id: uid.Bytes()}})
	// if err != nil {
	// 	return err
	// }

	// go func() {
	// 	for {
	// 		select {
	// 		case <-ctx.Context.Done():
	// 			return
	// 		default:
	// 		}

	// 		m, err := w.Recv()
	// 		if err == io.EOF {
	// 			log.Println("EOF received")
	// 			return
	// 		} else if err != nil {
	// 			log.Println("unable to receive message", err)
	// 			continue
	// 		}

	// 		log.Println("DERP", spew.Sdump(m))
	// 	}
	// }()

	go func() {
		makeevt := func(e *ffigraph.EventInfo) *events.Message {
			switch e.State {
			case ffigraph.Popped:
				return events.NewTaskCompleted(e.Parent, e.ID, "completed")
			case ffigraph.Pushed:
				return events.NewTaskInitiated(e.Parent, e.ID, "initiated")
			default:
				return events.NewTaskErrored(e.ID, fmt.Sprintf("unknown %d", e.State))
			}
		}

		c := events.NewEventsClient(cc)
		for {
			select {
			case <-gctx.Context.Done():
				return
			case evt := <-ebuf:
				if _, err := c.Dispatch(gctx.Context, events.NewDispatch(makeevt(evt))); err != nil {
					log.Println("unable to dispatch event", err, spew.Sdump(evt))
					continue
				}
			}
		}
	}()

	log.Println("cacheid", ws.CachedID)

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(gctx.Context)
	if err != nil {
		return err
	}

	modules, err := compile.FromTranspiled(gctx.Context, ws, roots...)
	if err != nil {
		return err
	}

	if len(modules) == 0 {
		return errors.New("no usable modules detected")
	}

	log.Println("modules", modules)
	if err = runners.BuildRootContainerPath(gctx.Context, t.Dir, filepath.Join(ws.RuntimeDir, "Containerfile")); err != nil {
		return err
	}

	if err = wasix.WarmCacheDirectory(gctx.Context, filepath.Join(ws.Root, ws.BuildDir), wasix.WazCacheDir(filepath.Join(ws.Root, ws.RuntimeDir))); err != nil {
		log.Println("unable to prewarm wasi directory cache", err)
	}

	runner := pc8s.NewProxyClient(cc)

	for _, m := range modules {
		options := []string{}
		if t.Privileged {
			options = append(options, "--privileged")
		}

		options = append(options,
			"--volume", runners.AgentMountReadOnly(m.Path, "/opt/egmodule.wasm"),
		)

		_, err = runner.Module(gctx.Context, &pc8s.ModuleRequest{
			Image:   "eg",
			Name:    fmt.Sprintf("eg-%s", uid.String()),
			Mdir:    ws.BuildDir,
			Options: options,
		})

		if err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	return nil
}
