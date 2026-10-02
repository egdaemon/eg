package egautogentest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeSamplePackage(t *testing.T) string {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/sample\n\ngo 1.25\n"), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "sample.go"), []byte(`package sample

type Account struct {
	Name string
}

func Greet(a Account) string {
	return "hello " + a.Name
}

func (t *Account) Rename(n string) {
	t.Name = n
}
`), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

type fixture struct{}

func TestGreet(t *testing.T) {
	Greet(Account{Name: "world"})
}
`), 0644))

	return dir
}

func TestLoadPackage(t *testing.T) {
	pkg, err := loadPackage(writeSamplePackage(t))
	require.NoError(t, err)
	require.Equal(t, "sample", pkg.name)
	require.Len(t, pkg.files, 2)

	_, err = loadPackage(t.TempDir())
	require.Error(t, err)
}

func TestFindFuncDecl(t *testing.T) {
	pkg, err := loadPackage(writeSamplePackage(t))
	require.NoError(t, err)

	decl := findFuncDecl(pkg, "Greet")
	require.NotNil(t, decl)
	require.Equal(t, "Greet", decl.Name.Name)

	method := findFuncDecl(pkg, "Account.Rename")
	require.NotNil(t, method)
	require.Equal(t, "Rename", method.Name.Name)

	require.Nil(t, findFuncDecl(pkg, "Rename"))
	require.Nil(t, findFuncDecl(pkg, "DoesNotExist"))
}

func TestCollectTypes(t *testing.T) {
	pkg, err := loadPackage(writeSamplePackage(t))
	require.NoError(t, err)

	decl := findFuncDecl(pkg, "Greet")
	require.NotNil(t, decl)

	rendered := collectTypes(pkg, decl)
	require.Contains(t, rendered, "type Account struct")
	require.Contains(t, rendered, "Name string")
	require.NotContains(t, rendered, "fixture")
}

func TestCollectUsage(t *testing.T) {
	pkg, err := loadPackage(writeSamplePackage(t))
	require.NoError(t, err)

	usage := collectUsage(pkg, "Greet")
	require.Contains(t, usage, "func TestGreet")
	require.Contains(t, usage, "Greet(Account{Name: \"world\"})")

	require.Equal(t, "no existing usage of this function was found.", collectUsage(pkg, "DoesNotExist"))
}

func TestExtractTest(t *testing.T) {
	t.Run("fenced code block is extracted and forced into the package", func(t *testing.T) {
		response := "here are the tests:\n```go\npackage wrong\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {\n}\n\nfunc TestGreetEmpty(t *testing.T) {\n}\n\nfunc helper() {}\n```\nskipped: nothing"
		src, tests, err := extractTest(response, "sample")
		require.NoError(t, err)
		require.Equal(t, []string{"TestGreet", "TestGreetEmpty"}, tests)
		require.Contains(t, string(src), "package sample\n")
		require.NotContains(t, string(src), "```")
	})

	t.Run("the block containing tests is preferred", func(t *testing.T) {
		response := "```bash\ngo test ./...\n```\n```go\npackage sample\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {}\n```"
		_, tests, err := extractTest(response, "sample")
		require.NoError(t, err)
		require.Equal(t, []string{"TestGreet"}, tests)
	})

	t.Run("unfenced source is accepted", func(t *testing.T) {
		_, tests, err := extractTest("package sample\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {}\n", "sample")
		require.NoError(t, err)
		require.Equal(t, []string{"TestGreet"}, tests)
	})

	t.Run("responses without tests are rejected", func(t *testing.T) {
		_, _, err := extractTest("```go\npackage sample\n\nfunc helper() {}\n```", "sample")
		require.Error(t, err)
	})

	t.Run("invalid go is rejected", func(t *testing.T) {
		_, _, err := extractTest("i couldn't write a test for this", "sample")
		require.Error(t, err)
	})
}
