package coverage

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Masterminds/squirrel"

	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/internal/squirrelx"
)

// extensions of the files of each language, the key is the lower case name of the language.
var languages = map[string][]string{
	"c":          {".c", ".h"},
	"cpp":        {".cc", ".cpp", ".cxx", ".hh", ".hpp"},
	"csharp":     {".cs"},
	"dart":       {".dart"},
	"go":         {".go"},
	"java":       {".java"},
	"javascript": {".cjs", ".js", ".jsx", ".mjs"},
	"kotlin":     {".kt"},
	"php":        {".php"},
	"python":     {".py"},
	"ruby":       {".rb"},
	"rust":       {".rs"},
	"swift":      {".swift"},
	"typescript": {".ts", ".tsx"},
}

// Filter narrows the functions selected from the recorded coverage, filters given together all apply.
// they are applied before the number of functions is limited, so the limit is of the functions that match.
type Filter = squirrel.Sqlizer

// ReportSearchBuilder selects the functions with recorded coverage matching the filters.
func ReportSearchBuilder(filters ...Filter) squirrel.SelectBuilder {
	return squirrelx.PSQL.Select(sqlx.Columns(ReportScannerStaticColumns)...).From("'eg.metrics.coverage'").Where(
		squirrel.NotEq{"fnname": ""},
	).Where(squirrel.And(filters))
}

// ReportSearch runs the query and returns a scanner for the results.
func ReportSearch(ctx context.Context, q sqlx.Queryer, b squirrel.SelectBuilder) ReportScanner {
	return NewReportScannerStatic(b.RunWith(q).QueryContext(ctx))
}

// WorstSearchBuilder selects the functions matching the filters, the lowest statement coverage first and the lowest
// hit count of those equally covered. it is not limited.
func WorstSearchBuilder(filters ...Filter) squirrel.SelectBuilder {
	return ReportSearchBuilder(filters...).OrderBy("statements ASC", "hits ASC", "RANDOM()")
}

// SampleSearchBuilder selects the functions matching the filters in a random order. it is not limited.
func SampleSearchBuilder(filters ...Filter) squirrel.SelectBuilder {
	return ReportSearchBuilder(filters...).OrderBy("random()")
}

// QueryLanguage matches the functions of files in any of the languages, e.g.) "go". it is matched
// on the extension of the file. the name is not case sensitive. an unknown language is an error
// when the query is built. ignored when there are no names.
func QueryLanguage(names ...string) squirrel.Sqlizer {
	if len(names) == 0 {
		return squirrelx.Noop{}
	}

	return language(names)
}

type language []string

func (t language) ToSql() (string, []interface{}, error) {
	globs := make([]string, 0, len(t))
	for _, name := range t {
		exts, ok := languages[strings.ToLower(name)]
		if !ok {
			known := make([]string, 0, len(languages))
			for k := range languages {
				known = append(known, k)
			}
			sort.Strings(known)

			return "", nil, fmt.Errorf("unknown language %q, expected one of: %s", name, strings.Join(known, ", "))
		}

		for _, ext := range exts {
			globs = append(globs, "*"+ext)
		}
	}

	return anyGlob(globs).ToSql()
}

// QueryPaths matches the functions of files matching any of the glob patterns.
// patterns match the whole recorded path, which is relative to the workspace, `*` matches any run of
// characters including separators, `?` any single character, and `[a-z]` any character of the class.
// ignored when there are no patterns.
func QueryPaths(globs ...string) squirrel.Sqlizer {
	if len(globs) == 0 {
		return squirrelx.Noop{}
	}

	return anyGlob(globs)
}

// QueryExcludePaths removes the functions of files matching any of the glob patterns, see QueryPaths.
// ignored when there are no patterns.
func QueryExcludePaths(globs ...string) squirrel.Sqlizer {
	if len(globs) == 0 {
		return squirrelx.Noop{}
	}

	return squirrel.Expr("NOT (?)", anyGlob(globs))
}

// QueryCoverage matches the functions whose statement coverage is within min and max percent, both inclusive.
// the range is not validated, the database handles values it does not accept.
func QueryCoverage(min, max float32) squirrel.Sqlizer {
	return squirrel.Expr("statements BETWEEN ? AND ?", min, max)
}

// anyGlob is the disjunction of the glob patterns.
func anyGlob(globs []string) squirrel.Sqlizer {
	terms := make(squirrel.Or, 0, len(globs))
	for _, glob := range slices.Compact(slices.Sorted(slices.Values(globs))) {
		terms = append(terms, squirrel.Expr("path GLOB ?", glob))
	}

	return terms
}
