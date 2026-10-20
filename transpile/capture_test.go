package transpile

import (
	"testing"

	"github.com/dave/jennifer/jen"
	"github.com/stretchr/testify/require"
)

func TestRenderModule(t *testing.T) {
	t.Run("should format valid source", func(t *testing.T) {
		f := jen.NewFile("main")
		f.Func().Id("main").Params().Block(jen.Id("x").Op(":=").Lit(1), jen.Id("_").Op("=").Id("x"))

		src, err := renderModule(f)
		require.NoError(t, err)
		require.Contains(t, string(src), "func main() {\n\tx := 1\n\t_ = x\n}")
	})

	t.Run("should report invalid source with line numbers", func(t *testing.T) {
		f := jen.NewFile("main")
		f.Func().Id("main").Params().Block(jen.Id("x").Op(":=").Id("(func() error literal)"))

		_, err := renderModule(f)
		require.ErrorContains(t, err, "generated module is invalid")
		require.ErrorContains(t, err, "   3 | ")
		require.ErrorContains(t, err, "(func() error literal)")
	})
}
