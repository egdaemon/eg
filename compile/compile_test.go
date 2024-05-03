package compile_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("FromTranspiled", func() {
	It("should compile example 1", func(ctx context.Context) {
		var (
			err error
			ws  workspaces.Context
		)

		tmpdir := testx.TempDir()

		Expect(fsx.CloneTree(tmpdir, "example.1", os.DirFS(testx.Fixture()))).To(Succeed())
		ws, err = workspaces.New(ctx, tmpdir, ".eg", "")
		Expect(err).To(Succeed())
		roots, err := transpile.Autodetect(transpile.New(ws)).Run(ctx)
		Expect(err).To(Succeed())

		modules, err := compile.FromTranspiled(ctx, ws, roots...)
		Expect(err).To(Succeed())
		Expect(modules).To(HaveLen(1))
		Expect(modules[0].Generated).To(BeFalse())
		Expect(modules[0].Path).To(Equal(filepath.Join(tmpdir, ws.BuildDir, "main.wasm")))
	})
})
