package generators

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"strings"

	"github.com/james-lawrence/genieql/internal/errorsx"
	"github.com/zieckey/goini"
)

// ParseCommentOptions parses a configuration and converts it into an array of options.
func ParseCommentOptions(comments *ast.CommentGroup) (*goini.INI, error) {
	const magicPrefix = `genieql.options:`

	ini := goini.New()

	scanner := bufio.NewScanner(strings.NewReader(comments.Text()))
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		local := goini.New()
		local.SetParseSection(true)

		text := scanner.Text()
		if !strings.HasPrefix(text, magicPrefix) {
			continue
		}

		text = strings.TrimSpace(strings.TrimPrefix(text, magicPrefix))

		if err := local.Parse([]byte(text), "||", "="); err != nil {
			return nil, errorsx.Wrap(err, "failed to parse comment configuration")
		}

		ini.Merge(local, true)
	}

	return ini, nil
}

// CommentOptionQueryLiteral - extracts a query from a comment.
func CommentOptionQueryLiteral(ini *goini.INI) (*ast.BasicLit, bool) {
	const queryOption = `query-literal`
	tmp, ok := ini.Get(queryOption)
	return &ast.BasicLit{
		Kind:  token.STRING,
		Value: fmt.Sprintf("`%s`", tmp),
	}, ok
}

// CommentOptionQueryConst - extracts a query from a comment.
func CommentOptionQueryConst(ini *goini.INI) (ast.Expr, bool) {
	const queryOption = `query-const`
	var (
		err error
		x   ast.Expr
		tmp string
		ok  bool
	)

	if tmp, ok = ini.Get(queryOption); !ok {
		return nil, false
	}

	if x, err = parser.ParseExpr(tmp); err != nil {
		log.Println("failed to parse query const expression", err)
		return x, false
	}

	return x, true
}

// CommentOptionQuery - extracts a query from a comment.
// uses either a query-literal or query-const returns false if both or neither are specified.
func CommentOptionQuery(ini *goini.INI) (ast.Expr, bool) {
	var (
		x          ast.Expr
		tmp        ast.Expr
		literal    bool
		queryConst bool
	)

	if tmp, literal = CommentOptionQueryLiteral(ini); literal {
		x = tmp
	}

	if tmp, queryConst = CommentOptionQueryConst(ini); queryConst {
		x = tmp
	}

	return x, literal != queryConst
}

// CommentOptionDefaultColumns - extracts columns that should be defaulted from
// a comment
func CommentOptionDefaultColumns(ini *goini.INI) ([]string, bool) {
	const defaultColumns = `default-columns`
	tmp, ok := ini.Get(defaultColumns)
	return strings.Split(tmp, ","), ok
}

// CommentOptionTable - specifies a table for a batch insert function declaration.
func CommentOptionTable(ini *goini.INI) (string, bool) {
	const tableName = `table`
	return ini.Get(tableName)
}
