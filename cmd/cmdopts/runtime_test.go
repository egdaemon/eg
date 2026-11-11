package cmdopts_test

import (
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/stretchr/testify/require"
)

func TestRuntimeValidate(t *testing.T) {
	const toc = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	t.Run("podman needs no egiso", func(t *testing.T) {
		require.NoError(t, cmdopts.Runtime{Runtime: eg.RuntimePodman}.Validate())
	})

	t.Run("podman ignores an egiso", func(t *testing.T) {
		require.NoError(t, cmdopts.Runtime{Runtime: eg.RuntimePodman, Egiso: "root.egiso", TOC: "invalid"}.Validate())
	})

	t.Run("vm with an egiso and its toc", func(t *testing.T) {
		require.NoError(t, cmdopts.Runtime{Runtime: eg.RuntimeVM, Egiso: "https://example.com/root.egiso", TOC: toc}.Validate())
	})

	t.Run("vm without an egiso", func(t *testing.T) {
		require.ErrorContains(t, cmdopts.Runtime{Runtime: eg.RuntimeVM, TOC: toc}.Validate(), "--egiso and --toc")
	})

	t.Run("vm without a toc", func(t *testing.T) {
		require.ErrorContains(t, cmdopts.Runtime{Runtime: eg.RuntimeVM, Egiso: "root.egiso"}.Validate(), "--egiso and --toc")
	})

	t.Run("vm with a toc that is not a digest", func(t *testing.T) {
		require.Error(t, cmdopts.Runtime{Runtime: eg.RuntimeVM, Egiso: "root.egiso", TOC: "e3b0c442"}.Validate())
	})
}

func TestRuntimeEnviron(t *testing.T) {
	const toc = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

	t.Run("names the vm runtime, its egiso and its toc", func(t *testing.T) {
		environ := envx.NewEnvironFromStrings(cmdopts.Runtime{Runtime: eg.RuntimeVM, Egiso: "https://example.com/root.egiso", TOC: toc}.Environ()...)
		require.Equal(t, eg.RuntimeVM, environ.String("", eg.EnvComputeRuntime))
		require.Equal(t, "https://example.com/root.egiso", environ.String("", eg.EnvComputeEgiso))
		require.Equal(t, toc, environ.String("", eg.EnvComputeEgisoTOC))
	})

	t.Run("names nothing for podman", func(t *testing.T) {
		require.Empty(t, cmdopts.Runtime{Runtime: eg.RuntimePodman, Egiso: "https://example.com/root.egiso", TOC: toc}.Environ())
	})
}
