package compute

type Cmd struct {
	Builtin    cmdbuiltin  `cmd:"" help:"EXPERIMENTAL: run builtin workloads like cache clearing and standalone containers/wasi workloads"`
	Cache      cmdCache    `cmd:"" help:"manage the runner workload cache"`
	Archive    archive     `cmd:"" help:"compiles and creates a workload tar archive from manual inspection/storage"`
	Spooled    spooled     `cmd:"" help:"EXPERIMENTAL: run a workload archive read from stdin through the daemon's spool and queue, i.e. eg compute archive <module> | eg compute spooled"`
	Upload     upload      `cmd:"" help:"compiles and uploads a workload to the cluster"`
	Watch      watch       `cmd:"" help:"follow the logs and result of an enqueued workload by id"`
	Download   download    `cmd:"" help:"download the logs or analytics of a workload by id"`
	Cancel     cancel      `cmd:"" help:"cancel an enqueued or running workload by id"`
	Local      local       `cmd:"" help:"execute the interpreter on the given directory"`
	Baremetal  baremetal   `cmd:"" help:"execute the interpreter on the given directory within the host itself, inherently unsafe"`
	Serve      serve       `cmd:"" help:"run a service within eg, automiacally uses builds and loads the containerfile without the module directory and .eg.env if it exists"`
	Sessions   sessionsCmd `cmd:"" name:"sessions" help:"list the workloads served with 'eg compute serve --ssh', running and previous"`
	LAN        lanCmd      `cmd:"" name:"lan" help:"manage compute systems on your local network"`
	Pub        pubCmd      `cmd:"" name:"pub" help:"manage compute systems on your account"`
	History    historyCmd  `cmd:"" name:"history" help:"list the completed workloads of the account, result is ✔ completed, ✖ failed or ⏹ cancelled"`
	Ls         lsCmd       `cmd:"" name:"ls" help:"list the active workloads of the account, state is ⏸ queued or ▶ running"`
	Containers c8scmds     `cmd:"" name:"containers" aliases:"c8s" help:"EXPERIMENTAL: build and upload a container file workload to the cluster"`
}
