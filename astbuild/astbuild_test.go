package astbuild_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strconv"
	"testing"

	"github.com/egdaemon/eg/astbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpr(t *testing.T) {
	for _, tc := range []string{
		"foo",
		"foo.Bar",
		"foo(a, b)",
		"a + b * c",
		`"literal"`,
		"[]string(nil)",
	} {
		t.Run(tc, func(t *testing.T) {
			expr := astbuild.Expr(tc)
			require.NotNil(t, expr)
			assert.Equal(t, tc, types.ExprString(expr))
		})
	}

	t.Run("invalid templates panic", func(t *testing.T) {
		assert.Panics(t, func() { astbuild.Expr("foo(") })
	})
}

func TestField(t *testing.T) {
	f := astbuild.Field(ast.NewIdent("int"), ast.NewIdent("a"), ast.NewIdent("b"))
	require.NotNil(t, f)
	assert.Equal(t, "int", types.ExprString(f.Type))
	require.Len(t, f.Names, 2)
	assert.Equal(t, "a", f.Names[0].Name)
	assert.Equal(t, "b", f.Names[1].Name)

	t.Run("anonymous", func(t *testing.T) {
		f := astbuild.Field(ast.NewIdent("error"))
		assert.Empty(t, f.Names)
	})
}

func TestFieldList(t *testing.T) {
	a := astbuild.Field(ast.NewIdent("int"), ast.NewIdent("a"))
	b := astbuild.Field(ast.NewIdent("string"), ast.NewIdent("b"))

	l := astbuild.FieldList(a, b)
	require.NotNil(t, l)
	assert.Equal(t, []*ast.Field{a, b}, l.List)
	assert.Equal(t, 2, l.NumFields())

	assert.Equal(t, 0, astbuild.FieldList().NumFields())
}

func TestCallExpr(t *testing.T) {
	call := astbuild.CallExpr(astbuild.SelExpr("fmt", "Println"), astbuild.StringLiteral("hello"), astbuild.IntegerLiteral(1))
	assert.Equal(t, "fmt.Println(`hello`, 1)", types.ExprString(call))

	t.Run("no arguments", func(t *testing.T) {
		assert.Equal(t, "run()", types.ExprString(astbuild.CallExpr(ast.NewIdent("run"))))
	})
}

func TestSelExpr(t *testing.T) {
	sel := astbuild.SelExpr("eg", "Module")
	assert.Equal(t, "eg.Module", types.ExprString(sel))
	assert.Equal(t, "eg", types.ExprString(sel.X))
	assert.Equal(t, "Module", sel.Sel.Name)
}

func TestIntegerLiteral(t *testing.T) {
	for _, n := range []int{0, 1, 42, -7} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			lit := astbuild.IntegerLiteral(n)
			assert.Equal(t, token.INT, lit.Kind)
			assert.Equal(t, strconv.Itoa(n), lit.Value)
		})
	}
}

func TestBinaryExpr(t *testing.T) {
	expr := astbuild.BinaryExpr(ast.NewIdent("a"), token.ADD, astbuild.IntegerLiteral(1))
	assert.Equal(t, "a + 1", types.ExprString(expr))

	t.Run("nested", func(t *testing.T) {
		expr := astbuild.BinaryExpr(
			astbuild.BinaryExpr(ast.NewIdent("a"), token.MUL, ast.NewIdent("b")),
			token.EQL,
			astbuild.IntegerLiteral(2),
		)
		assert.Equal(t, "a * b == 2", types.ExprString(expr))
	})
}

func TestStringLiteral(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "simple", value: "hello world"},
		{name: "path", value: "main.wasm.d/module.19.3.wasm"},
		{name: "quotes", value: `say "hi"`},
		{name: "backslash", value: `a\b`},
		{name: "escape sequence text", value: `a\nb`},
		{name: "newline", value: "a\nb"},
		// raw string literals cannot contain a backtick.
		{name: "backtick", value: "a`b"},
		// a carriage return inside of a raw string literal is discarded by the parser.
		{name: "carriage return", value: "a\rb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lit := astbuild.StringLiteral(tc.value)
			assert.Equal(t, token.STRING, lit.Kind)

			// the literal must be valid go source that evaluates to the original value.
			parsed, err := parser.ParseExpr(lit.Value)
			require.NoError(t, err, "literal %s is not valid go", lit.Value)

			basic, ok := parsed.(*ast.BasicLit)
			require.True(t, ok, "literal %s did not parse as a single string", lit.Value)

			actual, err := strconv.Unquote(basic.Value)
			require.NoError(t, err)
			assert.Equal(t, tc.value, actual)
		})
	}
}
