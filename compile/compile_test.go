package compile_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/egdaemon/eg"
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
		ws, err = workspaces.New(ctx, tmpdir, eg.DefaultModuleDirectory(), "")
		Expect(err).To(Succeed())
		roots, err := transpile.Autodetect(transpile.New(ws)).Run(ctx)
		Expect(err).To(Succeed())
		err = compile.EnsureRequiredPackages(ctx, filepath.Join(ws.Root, ws.TransDir))
		Expect(err).To(Succeed())
		modules, err := compile.FromTranspiled(ctx, ws, roots...)
		Expect(err).To(Succeed())
		Expect(modules).To(HaveLen(1))
		Expect(modules[0].Generated).To(BeFalse())
		Expect(modules[0].Path).To(Equal(filepath.Join(tmpdir, ws.BuildDir, "main.wasm")))
	})

	It("should transform nested modules", func(ctx context.Context) {
		var (
			err error
			ws  workspaces.Context
		)

		tmpdir := testx.TempDir()

		Expect(fsx.CloneTree(ctx, tmpdir, "example.2", os.DirFS(testx.Fixture()))).To(Succeed())
		ws, err = workspaces.New(ctx, tmpdir, eg.DefaultModuleDirectory(), "")
		Expect(err).To(Succeed())
		roots, err := transpile.Autodetect(transpile.New(ws)).Run(ctx)
		Expect(err).To(Succeed())
		err = compile.EnsureRequiredPackages(ctx, filepath.Join(ws.Root, ws.TransDir))
		Expect(err).To(Succeed())

		modules, err := compile.FromTranspiled(ctx, ws, roots...)

		Expect(err).To(Succeed())
		Expect(modules).To(HaveLen(1))
		Expect(modules[0].Generated).To(BeFalse())
		Expect(modules[0].Path).To(Equal(filepath.Join(tmpdir, ws.BuildDir, "main.wasm")))
		Expect(testx.ReadMD5(filepath.Join(tmpdir, ws.TransDir, "m1", "m1.go"))).To(Equal("6d5e29ce-6e99-d52f-f8c6-4ab44bee50b1"), testx.ReadString(filepath.Join(tmpdir, ws.TransDir, "m1", "m1.go")))
		Expect(testx.ReadMD5(filepath.Join(tmpdir, ws.TransDir, "m1", "m2", "m2.go"))).To(Equal("8d6b4444-b948-e467-8435-24d7c4fea235"))
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
		ws, err = workspaces.New(ctx, tmpdir, eg.DefaultModuleDirectory(), "")
		Expect(err).To(Succeed())
		roots, err := transpile.Autodetect(transpile.New(ws)).Run(ctx)
		Expect(err).To(Succeed())
		err = compile.EnsureRequiredPackages(ctx, filepath.Join(ws.Root, ws.TransDir))
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
