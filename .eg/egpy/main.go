// egpyenv/bin/python -m grpc_tools.protoc -I ../.proto --python_out=. --pyi_out=. --grpc_python_out=metrics ../.proto/eg.interp.events.proto
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"eg/ci/debian"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/wasi/eggit"
	"github.com/egdaemon/eg/runtime/wasi/shell"
)

func Build(ctx context.Context, _ eg.Op) error {
	c := eggit.EnvCommit()
	runtime := shell.Runtime().Directory("egpylib").
		Environ("PYPI_USERNAME", "__token__").
		Environ("PYPI_PASSWORD", envx.String("", "PYPI_PASSWORD")).
		Environ("EGPY_VERSION", fmt.Sprintf("0.1.%d", c.Committer.When.UnixMilli()))

	return shell.Run(
		ctx,
		runtime.New("pwd"),
		runtime.New("env"),
		runtime.New("tree -L 1"),
		runtime.New("python3 -m venv .egpyenv"),
		runtime.New(".egpyenv/bin/pip install --upgrade build twine"),
		runtime.New(".egpyenv/bin/pip install ."),
		runtime.New(".egpyenv/bin/python -m grpc_tools.protoc -I ../.proto --python_out=egpy --pyi_out=egpy --grpc_python_out=egpy/metrics ../.proto/eg.interp.events.proto"),
		runtime.New(".egpyenv/bin/python -m build"),
		runtime.New(".egpyenv/bin/python -m twine upload -u ${PYPI_USERNAME} -p ${PYPI_PASSWORD} dist/*"),
	)
}

func main() {
	ctx, done := context.WithTimeout(context.Background(), time.Hour)
	defer done()

	err := eg.Perform(
		ctx,
		eggit.AutoClone,
		eg.Parallel(
			eg.Build(eg.Container(debian.ContainerName).BuildFromFile(".dist/deb/Containerfile")),
		),
		eg.Parallel(
			eg.Module(ctx, eg.Container(debian.ContainerName), Build),
		),
	)

	if err != nil {
		log.Fatalln(err)
	}
}
