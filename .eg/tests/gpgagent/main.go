package main

import (
	"context"
	"log"

	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/egdaemon/eg/runtime/wasi/shell"
	"github.com/egdaemon/eg/runtime/x/wasi/egbug"
)

func Debug(runtime shell.Command) eg.OpFn {
	return shell.Op(
		runtime.New("env | grep -Ev \"HOME=|LOGNAME=|USER=\" | sort | md5sum"),
		runtime.New("env | grep -Ev \"HOME=|LOGNAME=|USER=\""),
		runtime.New("env | grep -i GNUPGHOME"),
		runtime.New("ps aux"),
		runtime.New("cat ${GNUPGHOME}/gpg-agent.conf").Lenient(true),
		runtime.New("ls -lha ${GNUPGHOME}").Lenient(true),
		runtime.New("gpgconf --list-dirs"),
		runtime.New("ls -lha /run/user/0/").Lenient(true),
	)
}

// ensure the gpg agent environment is working.
func Test(ctx context.Context, op eg.Op) error {
	runtime := shell.Runtime().Privileged()
	debug := eg.Sequential(
		egbug.Log("---------------------------- failed ----------------------------"),
		egbug.Log("----------------------------  egd   ----------------------------"),
		Debug(shell.Runtime()),
		egbug.Log("----------------------------  root  ----------------------------"),
		Debug(shell.Runtime().Privileged()),
	)
	return eg.Sequential(
		egbug.DebugFailure(
			// ensure that gpg home is set to the correct directory.
			shell.Op(runtime.New("test ${GNUPGHOME} = /eg.mnt/.gnupg")),
			debug,
		),
		egbug.DebugFailure(
			// ensure that gpg home is pointing to the correct directory.
			shell.Op(runtime.New("test $(gpgconf --list-dirs agent-socket) = /eg.mnt/.gnupg/S.gpg-agent")),
			debug,
		),
		egbug.DebugFailure(
			// ensure that gpg uses the agent provided by the host.
			shell.Op(runtime.New("test \"$(gpg-connect-agent --no-autostart /bye 2>&1)\" = \"\"").Privileged()),
			debug,
		),
		shell.Op(
			runtime.New("gpgconf --list-dirs"),
			runtime.New("ls -lha /run/user/0/gnupg/"),
			runtime.New("tree /run/user/0/gnupg/"),
		),
	)(ctx, op)
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), egenv.TTL())
	defer done()

	err := eg.Perform(
		ctx,
		eg.Build(eg.DefaultModule()),
		Test,
		eg.Module(ctx, eg.DefaultModule(), Test),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
