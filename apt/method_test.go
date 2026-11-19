package apt

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// runMethod drives the method over pipes with the input messages, returning every message it sent.
func runMethod(t *testing.T, srv *httptest.Server, input ...Message) []Message {
	t.Helper()

	var (
		accounts []string
		mu       sync.Mutex
	)

	m := &Method{Client: func(account string) (*http.Client, error) {
		mu.Lock()
		defer mu.Unlock()
		accounts = append(accounts, account)
		c := srv.Client()
		c.CheckRedirect = SameHostRedirects
		return c, nil
	}}

	inr, inw := io.Pipe()
	outr, outw := io.Pipe()

	errc := make(chan error, 1)
	go func() {
		errc <- m.Run(context.Background(), inr, outw)
		outw.Close()
	}()

	go func() {
		for _, msg := range input {
			_, _ = io.WriteString(inw, msg.String())
		}
		inw.Close()
	}()

	var out []Message
	r := bufio.NewReader(outr)
	for {
		msg, err := ReadMessage(r)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		out = append(out, msg)
	}

	require.NoError(t, <-errc)
	// the client is built once for the account however many uris are acquired.
	require.LessOrEqual(t, len(accounts), 1)
	return out
}

func egsuri(srv *httptest.Server, path string) string {
	return strings.Replace(srv.URL, "https://", Scheme+"://", 1) + path
}

func acquire(uri string, filename string, fields ...Field) Message {
	return Message{Code: 600, Text: "URI Acquire", Fields: append([]Field{{Key: "URI", Value: uri}, {Key: "Filename", Value: filename}}, fields...)}
}

func TestReadMessage(t *testing.T) {
	t.Run("should parse fields in order including repeated keys", func(t *testing.T) {
		r := bufio.NewReader(strings.NewReader("\n601 Configuration\nConfig-Item: a=1\nConfig-Item: b=2\n\n600 URI Acquire\nURI: egs://x/y\n"))

		m, err := ReadMessage(r)
		require.NoError(t, err)
		require.Equal(t, 601, m.Code)
		require.Equal(t, "Configuration", m.Text)
		require.Equal(t, []Field{{Key: "Config-Item", Value: "a=1"}, {Key: "Config-Item", Value: "b=2"}}, m.Fields)

		m, err = ReadMessage(r)
		require.NoError(t, err)
		require.Equal(t, 600, m.Code)
		require.Equal(t, "egs://x/y", m.Get("uri"))

		_, err = ReadMessage(r)
		require.ErrorIs(t, err, io.EOF)
	})

	t.Run("should reject an invalid status line", func(t *testing.T) {
		_, err := ReadMessage(bufio.NewReader(strings.NewReader("nope\n\n")))
		require.Error(t, err)
	})
}

func TestMethod(t *testing.T) {
	const body = "Origin: eg\nSuite: stable\n"
	lastmod := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			http.Error(w, "unexpected authorization", http.StatusBadRequest)
			return
		}

		switch r.URL.Path {
		case "/repo/dists/stable/InRelease":
			if r.Header.Get("If-Modified-Since") == lastmod {
				w.WriteHeader(http.StatusNotModified)
				return
			}
			w.Header().Set("Last-Modified", lastmod)
			fmt.Fprint(w, body)
		case "/repo/pool/a.deb", "/repo/pool/b.deb", "/repo/pool/c.deb":
			fmt.Fprint(w, r.URL.Path)
		case "/repo/denied":
			http.Error(w, "nope", http.StatusUnauthorized)
		case "/repo/forbidden":
			http.Error(w, "nope", http.StatusForbidden)
		case "/repo/broken":
			http.Error(w, "nope", http.StatusBadGateway)
		case "/repo/redirect":
			http.Redirect(w, r, "https://elsewhere.invalid/x", http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Run("should announce its capabilities first", func(t *testing.T) {
		out := runMethod(t, srv)
		require.Len(t, out, 1)
		require.Equal(t, 100, out[0].Code)
		require.Equal(t, "1.2", out[0].Get("Version"))
		require.Equal(t, "true", out[0].Get("Pipeline"))
		require.Equal(t, "true", out[0].Get("Send-Config"))
	})

	t.Run("should download the file and report its hashes", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "InRelease")
		uri := egsuri(srv, "/repo/dists/stable/InRelease")

		out := runMethod(t, srv,
			Message{Code: 601, Text: "Configuration", Fields: []Field{{Key: "Config-Item", Value: "Debug::Acquire::egs=false"}}},
			acquire(uri, dst),
		)
		require.Len(t, out, 3)

		require.Equal(t, 200, out[1].Code)
		require.Equal(t, uri, out[1].Get("URI"))
		require.Equal(t, lastmod, out[1].Get("Last-Modified"))

		digest := sha256.Sum256([]byte(body))
		require.Equal(t, 201, out[2].Code)
		require.Equal(t, uri, out[2].Get("URI"))
		require.Equal(t, dst, out[2].Get("Filename"))
		require.Equal(t, fmt.Sprint(len(body)), out[2].Get("Size"))
		require.Equal(t, hex.EncodeToString(digest[:]), out[2].Get("SHA256-Hash"))
		require.NotEmpty(t, out[2].Get("SHA512-Hash"))
		require.NotEmpty(t, out[2].Get("SHA1-Hash"))
		require.NotEmpty(t, out[2].Get("MD5Sum-Hash"))
		require.Equal(t, lastmod, out[2].Get("Last-Modified"))

		written, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, body, string(written))

		info, err := os.Stat(dst)
		require.NoError(t, err)
		require.Equal(t, lastmod, info.ModTime().UTC().Format(http.TimeFormat))
	})

	t.Run("should report an ims hit when not modified", func(t *testing.T) {
		dst := filepath.Join(t.TempDir(), "InRelease")
		uri := egsuri(srv, "/repo/dists/stable/InRelease")

		out := runMethod(t, srv, acquire(uri, dst, Field{Key: "Last-Modified", Value: lastmod}))
		require.Len(t, out, 2)
		require.Equal(t, 201, out[1].Code)
		require.Equal(t, "true", out[1].Get("IMS-Hit"))
		require.Equal(t, dst, out[1].Get("Filename"))
		require.NoFileExists(t, dst)
	})

	t.Run("should fail permanently when unauthorized", func(t *testing.T) {
		for _, path := range []string{"/repo/denied", "/repo/forbidden"} {
			uri := egsuri(srv, path)
			out := runMethod(t, srv, acquire(uri, filepath.Join(t.TempDir(), "x")))
			require.Len(t, out, 2)
			require.Equal(t, 400, out[1].Code)
			require.Equal(t, "URI Failure", out[1].Text)
			require.Equal(t, uri, out[1].Get("URI"))
			require.Contains(t, out[1].Get("Message"), "not authorized")
			require.Empty(t, out[1].Get("Transient-Failure"))
		}
	})

	t.Run("should fail transiently on server errors", func(t *testing.T) {
		out := runMethod(t, srv, acquire(egsuri(srv, "/repo/broken"), filepath.Join(t.TempDir(), "x")))
		require.Len(t, out, 2)
		require.Equal(t, 400, out[1].Code)
		require.Equal(t, "HttpError502", out[1].Get("FailReason"))
		require.Equal(t, "true", out[1].Get("Transient-Failure"))
	})

	t.Run("should fail on a missing file", func(t *testing.T) {
		out := runMethod(t, srv, acquire(egsuri(srv, "/repo/missing"), filepath.Join(t.TempDir(), "x")))
		require.Len(t, out, 2)
		require.Equal(t, 400, out[1].Code)
		require.Equal(t, "HttpError404", out[1].Get("FailReason"))
		require.Empty(t, out[1].Get("Transient-Failure"))
	})

	t.Run("should refuse redirects to another host", func(t *testing.T) {
		out := runMethod(t, srv, acquire(egsuri(srv, "/repo/redirect"), filepath.Join(t.TempDir(), "x")))
		require.Len(t, out, 2)
		require.Equal(t, 400, out[1].Code)
		require.Contains(t, out[1].Get("Message"), "refusing redirect")
	})

	t.Run("should reject uris of another scheme", func(t *testing.T) {
		out := runMethod(t, srv, acquire(srv.URL+"/repo/pool/a.deb", filepath.Join(t.TempDir(), "x")))
		require.Len(t, out, 2)
		require.Equal(t, 400, out[1].Code)
	})

	t.Run("should answer pipelined acquires in order", func(t *testing.T) {
		dir := t.TempDir()
		names := []string{"a", "b", "c"}
		var input []Message
		for _, n := range names {
			input = append(input, acquire(egsuri(srv, "/repo/pool/"+n+".deb"), filepath.Join(dir, n)))
		}

		out := runMethod(t, srv, input...)
		require.Len(t, out, 1+2*len(names))
		for i, n := range names {
			start, done := out[1+2*i], out[2+2*i]
			require.Equal(t, 200, start.Code)
			require.Equal(t, 201, done.Code)
			require.Equal(t, egsuri(srv, "/repo/pool/"+n+".deb"), done.Get("URI"))

			written, err := os.ReadFile(filepath.Join(dir, n))
			require.NoError(t, err)
			require.Equal(t, "/repo/pool/"+n+".deb", string(written))
		}
	})
}
