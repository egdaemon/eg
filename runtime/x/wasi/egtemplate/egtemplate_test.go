package egtemplate_test

import (
	"io"
	"testing"
	"text/template"

	"github.com/egdaemon/eg/runtime/x/wasi/egtemplate"
	"github.com/stretchr/testify/require"
)

func TestReader(t *testing.T) {
	t.Run("renders", func(t *testing.T) {
		tmpl := template.Must(template.New("example").Parse("hello {{ .Name }}"))
		content, err := io.ReadAll(egtemplate.Reader(tmpl, struct{ Name string }{Name: "world"}))
		require.NoError(t, err)
		require.Equal(t, "hello world", string(content))
	})

	t.Run("reports render errors on read", func(t *testing.T) {
		tmpl := template.Must(template.New("example").Parse("hello {{ .Missing }}"))
		_, err := io.ReadAll(egtemplate.Reader(tmpl, struct{ Name string }{Name: "world"}))
		require.Error(t, err)
	})

	t.Run("defers rendering until read", func(t *testing.T) {
		value := "before"
		tmpl := template.Must(template.New("example").Funcs(template.FuncMap{
			"value": func() string { return value },
		}).Parse("{{ value }}"))
		r := egtemplate.Reader(tmpl, nil)
		value = "after"
		content, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "after", string(content))
	})
}
