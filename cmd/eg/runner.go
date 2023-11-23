package main

import (
	"embed"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davecgh/go-spew/spew"
	"github.com/gofrs/uuid"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/cmd/eg/daemons"
	"github.com/james-lawrence/eg/cmd/ux"
	"github.com/james-lawrence/eg/compile"
	"github.com/james-lawrence/eg/internal/envx"
	"github.com/james-lawrence/eg/interp"
	"github.com/james-lawrence/eg/interp/c8s"
	"github.com/james-lawrence/eg/interp/events"
	"github.com/james-lawrence/eg/interp/runtime/wasi/ffigraph"
	"github.com/james-lawrence/eg/runners"
	"github.com/james-lawrence/eg/runtime/wasi/langx"
	"github.com/james-lawrence/eg/transpile"
	"github.com/james-lawrence/eg/workspaces"
	"google.golang.org/grpc"
)

//go:embed DefaultContainerfile
var embedded embed.FS

func preparerootcontainer(cpath string) (err error) {
	var (
		c   fs.File
		dst *os.File
	)

	log.Println("default container path", cpath)
	if c, err = embedded.Open("DefaultContainerfile"); err != nil {
		return err
	}
	defer c.Close()

	if dst, err = os.OpenFile(cpath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600); err != nil {
		return err
	}

	if _, err = io.Copy(dst, c); err != nil {
		return err
	}

	return nil
}

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

	log.Println("DERP DERP", spew.Sdump(ws))
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

	rootc := filepath.Join(ws.RunnerDir, "Containerfile")

	if err = preparerootcontainer(rootc); err != nil {
		return err
	}

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(ctx.Context)
	if err != nil {
		return err
	}

	log.Println("cacheid", ws.CachedID)

	modules := make([]transpile.Compiled, 0, len(roots))

	for _, root := range roots {
		var (
			path string
		)

		if path, err = filepath.Rel(ws.TransDir, root.Path); err != nil {
			return err
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

		if err = compile.Run(ctx.Context, ws.ModuleDir, root.Path, path); err != nil {
			return err
		}
	}

	log.Println("modules", modules)
	{
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
