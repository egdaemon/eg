package c8sproxy

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/workspaces"
	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/require"
)

func TestServiceProxyModule(t *testing.T) {
	const (
		toc = digest.Digest("sha256:0000000000000000000000000000000000000000000000000000000000000000")
	)

	type ran struct {
		module   string
		location string
		toc      digest.Digest
		environ  []string
	}

	// proxy serves a workspace holding the module, recording the cells it runs.
	proxy := func(t *testing.T, options ...ServiceProxyOption) (*ProxyService, string, *[]ran) {
		ws := workspaces.Context{Root: t.TempDir(), BuildDir: "build"}
		module := filepath.Join(ws.Root, ws.BuildDir, "module.wasm")
		require.NoError(t, os.MkdirAll(filepath.Dir(module), 0700))
		require.NoError(t, os.WriteFile(module, nil, 0600))

		cells := &[]ran{}
		record := ServiceProxyOptionCell(func(ctx context.Context, module string, location string, toc digest.Digest, environ ...string) error {
			*cells = append(*cells, ran{module: module, location: location, toc: toc, environ: environ})
			return nil
		})
		return NewServiceProxy(log.Default(), ws, append([]ServiceProxyOption{record}, options...)...), module, cells
	}

	t.Run("runs a cell's module with the egiso the request names", func(t *testing.T) {
		p, module, cells := proxy(t, ServiceProxyOptionCommandEnviron("CI=true"))
		_, err := p.Module(t.Context(), &c8s.ModuleRequest{
			Image:   "https://example.com/root.egiso",
			Toc:     toc.String(),
			Module:  "module.wasm",
			Options: []string{"-e", "EG_COMPUTE_MODULE_NESTED_LEVEL=1", "--volume", "/a:/b", "--env", "A=1"},
		})
		require.NoError(t, err)
		require.Equal(t, []ran{{
			module:   module,
			location: "https://example.com/root.egiso",
			toc:      toc,
			environ:  []string{"CI=true", "EG_COMPUTE_MODULE_NESTED_LEVEL=1", "A=1"},
		}}, *cells)
	})

	t.Run("refuses an invalid digest", func(t *testing.T) {
		p, _, cells := proxy(t)
		_, err := p.Module(t.Context(), &c8s.ModuleRequest{Image: "root.egiso", Toc: "sha256:short", Module: "module.wasm"})
		require.ErrorContains(t, err, "invalid table of contents digest")
		require.Empty(t, *cells)
	})

	t.Run("refuses a cell when the runner does not support them", func(t *testing.T) {
		ws := workspaces.Context{Root: t.TempDir()}
		_, err := NewServiceProxy(log.Default(), ws).Module(t.Context(), &c8s.ModuleRequest{Image: "root.egiso", Toc: toc.String(), Module: "module.wasm"})
		require.ErrorContains(t, err, "cells are not supported")
	})
}
