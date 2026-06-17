package egautogentest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeSampleModule(t *testing.T) string {
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/sample\n\ngo 1.25\n"), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "sample.go"), []byte(`package sample

type Account struct {
	Name string
}

func Greet(a Account) string {
	return "hello " + a.Name
}
`), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "sample_test.go"), []byte(`package sample

import "testing"

func TestGreet(t *testing.T) {
	Greet(Account{Name: "world"})
}
`), 0644))

	return dir
}

func TestFindFuncDecl(t *testing.T) {
	dir := writeSampleModule(t)

	pkgs, err := loadPackages(dir)
	require.NoError(t, err)

	decl, fset, pkg := findFuncDecl(pkgs, "Greet")
	require.NotNil(t, decl)
	require.NotNil(t, fset)
	require.NotNil(t, pkg)
	require.Equal(t, "Greet", decl.Name.Name)

	missing, _, _ := findFuncDecl(pkgs, "DoesNotExist")
	require.Nil(t, missing)
}

func TestCollectTypes(t *testing.T) {
	dir := writeSampleModule(t)

	pkgs, err := loadPackages(dir)
	require.NoError(t, err)

	decl, fset, pkg := findFuncDecl(pkgs, "Greet")
	require.NotNil(t, decl)

	rendered := collectTypes(pkg, fset, decl)
	require.Contains(t, rendered, "type Account struct")
	require.Contains(t, rendered, "Name string")
}

func TestCollectUsage(t *testing.T) {
	dir := writeSampleModule(t)

	pkgs, err := loadPackages(dir)
	require.NoError(t, err)

	usage := collectUsage(pkgs, "Greet")
	require.Contains(t, usage, "func TestGreet")
	require.Contains(t, usage, "Greet(Account{Name: \"world\"})")

	require.Equal(t, "no existing usage of this function was found.", collectUsage(pkgs, "DoesNotExist"))
}
