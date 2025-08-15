package compute

type Cmd struct {
	Builtin    cmdbuiltin `cmd:"" help:"EXPERIMENTAL: run builtin workloads like cache clearing and standalone containers/wasi workloads"`
	Archive    archive    `cmd:"" help:"compiles and creates a workload tar archive from manual inspection/storage"`
	Upload     upload     `cmd:"" help:"compiles and uploads a workload to the cluster"`
	Local      local      `cmd:"" help:"execute the interpreter on the given directory"`
	Baremetal  baremetal  `cmd:"" help:"execute the interpreter on the given directory within the host itself, inherently unsafe"`
	Serve      serve      `cmd:"" help:"run a service within eg, automiacally uses builds and loads the containerfile without the module directory and .eg.env if it exists"`
	Containers c8scmds    `cmd:"" name:"containers" aliases:"c8s" help:"EXPERIMENTAL: build and upload a container file workload to the cluster"`
}
