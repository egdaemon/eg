package compute

type Cmd struct {
	Upload     upload    `cmd:"" help:"compiles and uploads a workload to the cluster"`
	Containers c8scmds   `cmd:"" name:"containers" aliases:"c8s" help:"build and upload a container file workload to the cluster"`
	Local      local     `cmd:"" help:"execute the interpreter on the given directory"`
	Baremetal  baremetal `cmd:"" help:"execute the interpreter on the given directory within the host itself, inherently unsafe"`
	Serve      serve     `cmd:"" help:"run a service within eg, automiacally uses builds and loads the containerfile without the module directory and .eg.env if it exists"`
}
