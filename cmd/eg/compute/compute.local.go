package compute

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
	"github.com/gofrs/uuid"
)

type local struct {
	cmdopts.RuntimeResources
	Dir            string   `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir      string   `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
	Name           string   `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:""`
	Privileged     bool     `name:"privileged" help:"run the initial container in privileged mode"`
	Dirty          bool     `name:"dirty" help:"include user directories and environment variables" hidden:"true"`
	MountEnvirons  string   `name:"environ" help:"environment file to pass to the module" default:""`
	EnvVars        []string `name:"env" short:"e" help:"environment variables to import" default:""`
	SSHAgent       bool     `name:"sshagent" help:"enable ssh agent" hidden:"true"`
	GPGAgent       bool     `name:"gpgagent" help:"enable gpg agent" hidden:"true"`
	GitRemote      string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference   string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_default_reference}"`
	ContainerCache string   `name:"croot" help:"container storage, ideally we'd be able to share with the host but for now" hidden:"true" default:"${vars_container_cache_directory}"`
}

func (t local) Run(gctx *cmdopts.Global) (err error) {
	var (
		homedir    = userx.HomeDirectoryOrDefault("/root")
		ws         workspaces.Context
		repo       *git.Repository
		uid        = uuid.Must(uuid.NewV7())
		environio  *os.File
		sshmount   runners.AgentOption = runners.AgentOptionNoop
		sshenvvar  runners.AgentOption = runners.AgentOptionNoop
		envvar     runners.AgentOption = runners.AgentOptionNoop
		mounthome  runners.AgentOption = runners.AgentOptionNoop
		privileged runners.AgentOption = runners.AgentOptionNoop
		gpgmount   runners.AgentOption = runners.AgentOptionNoop
		mountegbin runners.AgentOption = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
	)

	// TODO: create a kong bind variable to do this automatically and inject as needed.
	if err = fsx.MkDirs(0700, t.ContainerCache); err != nil {
		return errorsx.Wrap(err, "unable to setup container cache")
	}

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
		FromEnv(eg.EnvComputeContainerExec).
		FromEnviron(errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))...).
		Var("EG_INTERNAL_GIT_CLONE_ENABLED", strconv.FormatBool(false)) // hack to disable cloning

	if t.Dirty {
		mounthome = runners.AgentOptionAutoMountHome(homedir)
	} else if t.GPGAgent {
		gpgmount = runners.AgentOptionVolumes(
			runners.AgentMountOverlay(filepath.Join(homedir, ".gnupg"), "/root/.gnupg"),
		)
	}

	if t.SSHAgent {
		sshmount = runners.AgentOptionVolumes(
			runners.AgentMountOverlay(
				filepath.Join(homedir, ".ssh"),
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

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

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

	if t.Privileged {
		privileged = runners.AgentOptionCommandLine("--privileged")
	}

	debugx.Println("container cache", t.ContainerCache)

	// envx.Debug(os.Environ()...)
	ragent := runners.NewRunner(
		gctx.Context,
		ws,
		uid.String(),
		privileged,
		mounthome,
		envvar,
		sshmount,
		sshenvvar,
		gpgmount,
		mountegbin,
		runners.AgentOptionVolumes(
			runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.CacheDir), "/cache"),
			runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.RuntimeDir), "/opt/egruntime"),
			runners.AgentMountReadWrite(t.ContainerCache, "/var/lib/containers"),
		),
		runners.AgentOptionEnviron(environpath),
		runners.AgentOptionCommandLine("--env-file", environpath), // required for tty to work correct in local mode.
		runners.AgentOptionCommandLine("--cap-add", "NET_ADMIN"),  // required for loopback device creation inside the container
		runners.AgentOptionCommandLine("--cap-add", "SYS_ADMIN"),  // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
		runners.AgentOptionCommandLine("--device", "/dev/fuse"),
		runners.AgentOptionEnv(eg.EnvComputeRootModule, strconv.FormatBool(true)),
		runners.AgentOptionEnv(eg.EnvComputeModuleNestedLevel, strconv.Itoa(envx.Int(0, eg.EnvComputeModuleNestedLevel))),
		runners.AgentOptionEnv(eg.EnvComputeRunID, uid.String()),
		runners.AgentOptionEnv(eg.EnvComputeLoggingVerbosity, strconv.Itoa(gctx.Verbosity)),
	)

	for _, m := range modules {
		options := append(
			ragent.Options(),
			runners.AgentOptionVolumeSpecs(
				runners.AgentMountReadOnly(
					filepath.Join(ws.Root, ws.BuildDir, ws.Module, "main.wasm.d"),
					filepath.Join("/opt/egruntime", ws.Module, "main.wasm.d"),
				),
				runners.AgentMountReadOnly(m.Path, "/opt/egmodule.wasm"),
				runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.WorkingDir), eg.DefaultRootDirectory),
			)...)

		prepcmd := func(cmd *exec.Cmd) *exec.Cmd {
			cmd.Dir = ws.Root
			cmd.Stdout = log.Writer()
			cmd.Stderr = log.Writer()
			cmd.Stdin = os.Stdin
			return cmd
		}

		// TODO REVISIT using t.ws.RuntimeDir as moduledir.
		err := c8s.PodmanModule(gctx.Context, prepcmd, "eg", fmt.Sprintf("eg-%s", uid.String()), ws.RuntimeDir, options...)
		if err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	return nil
}
