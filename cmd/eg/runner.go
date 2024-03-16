package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/cmd/ux"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/tarx"
	"github.com/egdaemon/eg/interp"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/interp/runtime/wasi/ffigraph"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/runtime/wasi/langx"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
)

type runner struct {
	Dir        string `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir  string `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
	Name       string `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:""`
	Privileged bool   `name:"privileged" help:"run the initial container in privileged mode"`
	MountHome  bool   `name:"home" help:"mount home directory"`
}

func (t runner) Run(ctx *cmdopts.Global) (err error) {
	var (
		ws        workspaces.Context
		uid       = uuid.Must(uuid.NewV7())
		ebuf      = make(chan *ffigraph.EventInfo)
		cc        grpc.ClientConnInterface
		mounthome runners.AgentOption = runners.AgentOptionNoop
	)

	if ws, err = workspaces.New(ctx.Context, t.Dir, t.ModuleDir, t.Name); err != nil {
		return err
	}

	if t.MountHome {
		mounthome = runners.AgentOptionAutoMountHome(langx.Must(os.UserHomeDir()))
	}

	if cc, err = daemons.AutoRunnerClient(ctx, ws, uid.String(), mounthome); err != nil {
		return err
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
			case <-ctx.Context.Done():
				return
			case evt := <-ebuf:
				if _, err := c.Dispatch(ctx.Context, events.NewDispatch(makeevt(evt))); err != nil {
					log.Println("unable to dispatch event", err, spew.Sdump(evt))
					continue
				}
			}
		}
	}()

	log.Println("cacheid", ws.CachedID)

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(ctx.Context)
	if err != nil {
		return err
	}

	modules, err := compile.FromTranspiled(ctx.Context, ws, roots...)
	if err != nil {
		return err
	}

	log.Println("modules", modules)
	{
		rootc := filepath.Join(ws.RunnerDir, "Containerfile")

		if err = runners.PrepareRootContainer(rootc); err != nil {
			return err
		}

		cmd := exec.CommandContext(ctx.Context, "podman", "build", "--timestamp", "0", "-t", "eg", "-f", rootc, t.Dir)
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout

		if err = cmd.Run(); err != nil {
			return err
		}
	}

	runner := c8s.NewProxyClient(cc)

	for _, m := range modules {
		options := []string{}
		if t.Privileged {
			options = append(options, "--privileged")
		}
		options = append(options,
			"--env", "EG_BIN",
			"--volume", fmt.Sprintf("%s:/opt/egbin:ro", langx.Must(exec.LookPath(os.Args[0]))), // deprecated
			"--volume", fmt.Sprintf("%s:/opt/egmodule.wasm:ro", m.Path),
			"--volume", fmt.Sprintf("%s:/opt/eg:O", ws.Root),
		)
		_, err = runner.Module(ctx.Context, &c8s.ModuleRequest{
			Image:   "eg",
			Name:    fmt.Sprintf("eg-%s", uid.String()),
			Mdir:    ws.ModuleDir,
			Options: options,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

type module struct {
	Dir       string `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir string `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
	Module    string `arg:"" help:"name of the module to run"`
}

func (t module) Run(ctx *cmdopts.Global) (err error) {
	var (
		uid  = envx.String(uuid.Nil.String(), "EG_RUN_ID")
		ebuf = make(chan *ffigraph.EventInfo)
		cc   grpc.ClientConnInterface
	)

	// TODO: fill out workspaces...
	if cc, err = daemons.AutoRunnerClient(ctx, workspaces.Context{}, uid); err != nil {
		return err
	}
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
			case <-ctx.Context.Done():
				return
			case evt := <-ebuf:
				if _, err := c.Dispatch(ctx.Context, events.NewDispatch(makeevt(evt))); err != nil {
					log.Println("unable to dispatch event", err, spew.Sdump(evt))
					continue
				}
			}
		}
	}()

	// envx.Print(os.Environ())
	// fsx.PrintFS(os.DirFS("/opt/egruntime"))

	return interp.Remote(
		ctx.Context,
		uid,
		ffigraph.NewListener(ebuf),
		cc,
		t.Dir,
		t.Module,
		interp.OptionModuleDir(t.ModuleDir),
		interp.OptionRuntimeDir("/opt/egruntime"),
	)
}

type upload struct {
	SSHKeyPath  string        `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Dir         string        `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	ModuleDir   string        `name:"moduledir" help:"must be a subdirectory in the provided directory" default:".eg"`
	Name        string        `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:""`
	Environment []string      `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Dirty       bool          `name:"dirty" help:"include all environment variables"`
	Endpoint    string        `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/manager/" hidden:"true"`
	TTL         time.Duration `name:"ttl" help:"maximum runtime for the upload" default:"1h"`
	OS          string        `name:"os" help:"operating system the job requires" hidden:"true" default:"linux"`
	Arch        string        `name:"arch" help:"instruction set the job requires" hidden:"true" default:"${vars_arch}"`
	Cores       string        `name:"cores" help:"minimum number of cores the required" default:"${vars_cores_minimum_default}"`
	Memory      string        `name:"memory" help:"minimum amount of ram required" default:"${vars_memory_minimum_default}"`
	Labels      []string      `name:"labels" help:"custom labels required"`
}

func (t upload) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		signer               ssh.Signer
		ws                   workspaces.Context
		tmpdir               string
		archiveio, environio *os.File
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	if ws, err = workspaces.New(gctx.Context, t.Dir, t.ModuleDir, t.Name); err != nil {
		return err
	}

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(gctx.Context)
	if err != nil {
		return err
	}

	log.Println("cacheid", ws.CachedID)

	modules, err := compile.FromTranspiled(gctx.Context, ws, roots...)
	if err != nil {
		return err
	}
	log.Println("modules", modules)

	entry, found := slicesx.Find(func(c transpile.Compiled) bool {
		return !c.Generated
	}, modules...)

	if !found {
		return errors.New("unable to locate entry point")
	}

	if tmpdir, err = os.MkdirTemp("", "eg.upload.*"); err != nil {
		return errorsx.Wrap(err, "unable to  create temporary directory")
	}

	defer func() {
		errorsx.MaybeLog(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	if environio, err = os.Create(filepath.Join(tmpdir, "environ.env")); err != nil {
		return errorsx.Wrap(err, "unable to open the kernel archive")
	}
	defer environio.Close()

	if t.Dirty {
		for _, e := range os.Environ() {
			if _, err = fmt.Fprintf(environio, "%s\n", e); err != nil {
				return errorsx.Wrap(err, "unable to write environment variable")
			}
		}
	}

	for _, e := range t.Environment {
		if _, err = fmt.Fprintf(environio, "%s\n", e); err != nil {
			return errorsx.Wrap(err, "unable to write environment variable")
		}
	}

	if err = iox.Rewind(environio); err != nil {
		return errorsx.Wrap(err, "unable to rewind environment variables buffer")
	}

	if archiveio, err = os.CreateTemp(tmpdir, "kernel.*.tar.gz"); err != nil {
		return errorsx.Wrap(err, "unable to open the kernel archive")
	}
	defer archiveio.Close()

	if err = tarx.Pack(archiveio, filepath.Join(ws.Root, ws.BuildDir), environio.Name()); err != nil {
		return errorsx.Wrap(err, "unable to pack the kernel archive")
	}

	if err = iox.Rewind(archiveio); err != nil {
		return errorsx.Wrap(err, "unable to rewind kernel archive")
	}

	log.Println("archive", archiveio.Name())
	// if err = tarx.Inspect(archiveio); err != nil {
	// 	log.Println(errorsx.Wrap(err, "unable to inspect archive"))
	// }

	// TODO: determine the destination based on the requirements
	// i.e. cores, memory, labels, disk, videomem, etc.
	// not sure if the client should do this or the node we upload to.
	// if its the node we upload to it'll cost more due to having to
	// push the archive to another node that matches the requirements.
	// in theory we could use redirects to handle that but it'd still take a performance hit.
	mimetype, buf, err := httpx.Multipart(func(w *multipart.Writer) error {
		if err = w.WriteField("entrypoint", filepath.Base(entry.Path)); err != nil {
			return errorsx.Wrap(err, "unable to copy entry point")
		}

		if err = w.WriteField("ttl", t.TTL.String()); err != nil {
			return errorsx.Wrap(err, "unable to set ttl")
		}

		if err = w.WriteField("cores", t.Cores); err != nil {
			return errorsx.Wrap(err, "unable to set minimum cores")
		}

		if err = w.WriteField("memory", t.Memory); err != nil {
			return errorsx.Wrap(err, "unable to set minimum memory")
		}

		if err = w.WriteField("arch", t.Arch); err != nil {
			return errorsx.Wrap(err, "unable to isa architecture")
		}

		if err = w.WriteField("os", t.OS); err != nil {
			return errorsx.Wrap(err, "unable to operating system")
		}

		part, lerr := w.CreatePart(httpx.NewMultipartHeader("application/gzip", "archive", "kernel.tar.gz"))
		if lerr != nil {
			return errorsx.Wrap(lerr, "unable to create kernel part")
		}

		if _, lerr = io.Copy(part, archiveio); lerr != nil {
			return errorsx.Wrap(lerr, "unable to copy kernel")
		}

		return nil
	})
	if err != nil {
		return errorsx.Wrap(err, "unable to generate multipart upload")
	}

	chttp, err := authn.OAuth2SSHHTTPClient(
		context.WithValue(gctx.Context, oauth2.HTTPClient, tlsc.DefaultClient()),
		signer,
	)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(gctx.Context, http.MethodPost, t.Endpoint, buf)
	if err != nil {
		return errorsx.Wrap(err, "unable to create kernel upload request")
	}
	req.Header.Set("Content-Type", mimetype)

	resp, err := httpx.AsError(chttp.Do(req)) //nolint:golint,bodyclose
	defer httpx.TryClose(resp)

	if err != nil {
		return errorsx.Wrap(err, "unable to upload kernel for processing")
	}

	// TODO: monitoring the job once its uploaded and we have a run id.

	return nil
}

type monitor struct {
	RunID string `name:"runid"`
}

func (t monitor) Run(ctx *cmdopts.Global) (err error) {
	var (
		cc    *grpc.ClientConn
		grpcl net.Listener
	)

	if grpcl, err = daemons.DefaultAgentListener(); err != nil {
		return err
	}
	defer grpcl.Close()

	if err = daemons.Agent(ctx, grpcl); err != nil {
		return err
	}

	if cc, err = daemons.DefaultRunnerClient(ctx.Context); err != nil {
		return err
	}

	w, err := events.NewAgentClient(cc).Watch(ctx.Context, &events.RunWatchRequest{Run: &events.RunMetadata{Id: uuid.FromStringOrNil(t.RunID).Bytes()}})
	if err != nil {
		return err
	}

	p := tea.NewProgram(
		ux.NewGraph(),
		tea.WithoutSignalHandler(),
		tea.WithContext(ctx.Context),
	)

	go func() {
		for {
			select {
			case <-ctx.Context.Done():
				return
			default:
			}

			m, err := w.Recv()
			if err == io.EOF {
				log.Println("EOF received")
				return
			} else if err != nil {
				log.Println("unable to receive message", err)
				continue
			}

			p.Send(m)
		}
	}()

	go func() {
		<-ctx.Context.Done()
		p.Send(tea.Quit)
	}()

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
