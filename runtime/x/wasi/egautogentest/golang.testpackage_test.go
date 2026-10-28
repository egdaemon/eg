package egautogentest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTestPackageOf(t *testing.T) {
	type example struct {
		name       string
		dir        string
		fn         string
		expected   string
		importpath string
		external   bool
	}

	examples := []example{
		{name: "exported function", dir: ".fixtures/golang/example.1", fn: "Greet", expected: "sample_test", importpath: "example.com/sample", external: true},
		{name: "exported method of an exported type", dir: ".fixtures/golang/example.1", fn: "Account.Rename", expected: "sample_test", importpath: "example.com/sample", external: true},
		{name: "unexported function", dir: ".fixtures/golang/example.2", fn: "quiet", expected: "sample"},
		{name: "package within the module", dir: ".fixtures/golang/example.3/api", fn: "Equal", expected: "api_test", importpath: "example.com/mod/api", external: true},
		{name: "exported method of a generic type", dir: ".fixtures/golang/example.5", fn: "Generic.Method", expected: "sample_test", importpath: "example.com/sample", external: true},
		{name: "unexported method of an exported type", dir: ".fixtures/golang/example.5", fn: "Public.hidden", expected: "sample"},
		{name: "exported method of an unexported type", dir: ".fixtures/golang/example.5", fn: "hiddenType.Method", expected: "sample"},
		{name: "method of an unexported generic type", dir: ".fixtures/golang/example.5", fn: "hiddenGeneric.Value", expected: "sample"},
		{name: "unexported function beside exported ones", dir: ".fixtures/golang/example.5", fn: "unexported", expected: "sample"},
	}

	for _, e := range examples {
		t.Run(e.name, func(t *testing.T) {
			pkg, err := loadPackage(e.dir)
			require.NoError(t, err)

			decl := findFuncDecl(pkg, e.fn)
			require.NotNil(t, decl)

			testpackage, importpath, external := testPackageOf(e.dir, pkg, decl)
			require.Equal(t, e.expected, testpackage)
			require.Equal(t, e.importpath, importpath)
			require.Equal(t, e.external, external)
		})
	}
}
