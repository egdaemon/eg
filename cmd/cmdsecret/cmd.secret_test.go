package cmdsecret_test

import (
	"bytes"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdsecret"
	"github.com/egdaemon/eg/secrets"
	"github.com/stretchr/testify/require"
)

func runSecretCLI(t *testing.T, args []string) error {
	var cli struct {
		cmdopts.Global
		Secret cmdsecret.SecretCmd `cmd:""`
	}

	cli.Context = t.Context()

	parser, err := kong.New(&cli,
		kong.Name("eg"),
		kong.Bind(&cli.Global),
	)
	require.NoError(t, err)

	ctx, err := parser.Parse(args)
	if err != nil {
		return err
	}

	return ctx.Run()
}

func TestSecretCmd(t *testing.T) {
	t.Run("test update and read via cli", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "cli.chacha")
		pass := "clisecret"
		content := "hello-from-cli"
		uri := "chachasm://" + pass + "@" + path

		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		go func() {
			_, _ = w.Write([]byte(content))
			w.Close()
		}()

		err := runSecretCLI(t, []string{"secret", "update", uri})
		os.Stdin = oldStdin
		require.NoError(t, err)

		outputPath := filepath.Join(tmp, "output.txt")
		err = runSecretCLI(t, []string{"secret", "read", uri, "-o", outputPath})
		require.NoError(t, err)

		got, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		require.Equal(t, content, string(got))
	})

	t.Run("test update from file flag", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "file_input.chacha")
		inputPath := filepath.Join(tmp, "input.txt")
		content := "data-from-file"
		uri := "chachasm://pass@" + path

		require.NoError(t, os.WriteFile(inputPath, []byte(content), 0644))

		err := runSecretCLI(t, []string{"secret", "update", uri, "-i", inputPath})
		require.NoError(t, err)

		got, err := io.ReadAll(secrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, content, string(got))
	})

	t.Run("test read multiple uris to stdout", func(t *testing.T) {
		tmp := t.TempDir()
		uri1 := "chachasm://p@" + filepath.Join(tmp, "1.chacha")
		uri2 := "chachasm://p@" + filepath.Join(tmp, "2.chacha")

		require.NoError(t, secrets.Update(t.Context(), uri1, bytes.NewReader([]byte("one"))))
		require.NoError(t, secrets.Update(t.Context(), uri2, bytes.NewReader([]byte("two"))))

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := runSecretCLI(t, []string{"secret", "read", uri1, uri2})

		w.Close()
		os.Stdout = oldStdout
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = io.Copy(&buf, r)
		require.NoError(t, err)

		require.Equal(t, "one\ntwo\n", buf.String())
	})

	t.Run("encode stdin to stdout", func(t *testing.T) {
		content := []byte("hello encode")

		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		go func() {
			_, _ = w.Write(content)
			w.Close()
		}()

		oldStdout := os.Stdout
		or, ow, _ := os.Pipe()
		os.Stdout = ow

		err := runSecretCLI(t, []string{"secret", "b64"})

		ow.Close()
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = io.Copy(&buf, or)
		require.NoError(t, err)

		require.Equal(t, base64.URLEncoding.EncodeToString(content), buf.String())
	})

	t.Run("encode stdin to file", func(t *testing.T) {
		content := []byte("hello encode to file")
		outputPath := filepath.Join(t.TempDir(), "out.b64")

		oldStdin := os.Stdin
		r, w, _ := os.Pipe()
		os.Stdin = r
		go func() {
			_, _ = w.Write(content)
			w.Close()
		}()

		err := runSecretCLI(t, []string{"secret", "b64", "-o", outputPath})
		os.Stdin = oldStdin
		require.NoError(t, err)

		got, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		require.Equal(t, base64.URLEncoding.EncodeToString(content), string(got))
	})
}
