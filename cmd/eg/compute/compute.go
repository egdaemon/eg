package compute

type Cmd struct {
	Upload     upload `cmd:"" help:"compiles and uploads a workload to the cluster"`
	Containers c8s    `cmd:"" name:"containers" aliases:"c8s" help:"build and upload a container file workload to the cluster"`
}
