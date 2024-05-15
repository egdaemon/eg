package compile_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tetratelabs/wazero"
)

var _ = Describe("FromTranspiled", func() {
	It("should compile example 1", func(ctx context.Context) {
		var (
			err error
			ws  workspaces.Context
		)

		tmpdir := testx.TempDir()

		Expect(fsx.CloneTree(ctx, tmpdir, "example.1", os.DirFS(testx.Fixture()))).To(Succeed())
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

var _ = Describe("wasix warm cache", func() {
	It("should compile example 1 and warm the provided cache", func(ctx context.Context) {
		var (
			err error
			ws  workspaces.Context
		)

		tmpdir := testx.TempDir()

		wazcache, err := os.MkdirTemp(tmpdir, "wazcache")
		Expect(err).To(Succeed())

		Expect(fsx.CloneTree(ctx, tmpdir, "example.1", os.DirFS(testx.Fixture()))).To(Succeed())
		ws, err = workspaces.New(ctx, tmpdir, ".eg", "")
		Expect(err).To(Succeed())
		roots, err := transpile.Autodetect(transpile.New(ws)).Run(ctx)
		Expect(err).To(Succeed())

		_, err = compile.FromTranspiled(ctx, ws, roots...)
		Expect(err).To(Succeed())

		cache, err := wazero.NewCompilationCacheWithDir(wazcache)
		Expect(err).To(Succeed())
		defer cache.Close(ctx)
		Expect(
			wasix.WarmCacheFromDirectoryTree(ctx, filepath.Join(ws.Root, ws.BuildDir), cache),
		).To(Succeed())

		// grab the cache dir.
		cached, err := fs.ReadDir(os.DirFS(wazcache), ".")
		Expect(err).To(Succeed())
		Expect(cached).To(HaveLen(1))

		entries, err := fs.ReadDir(os.DirFS(wazcache), cached[0].Name())
		Expect(err).To(Succeed())
		Expect(entries).To(HaveLen(3))
	})
})
