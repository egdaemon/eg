package eghoneybadger_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"eg/compute/eghoneybadger"
)

func TestClientRecentAndLatest(t *testing.T) {
	var paths, queries []string
	var user, pass string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		queries = append(queries, r.URL.RawQuery)
		user, pass, _ = r.BasicAuth()

		switch r.URL.Path {
		case "/v2/projects/p1/faults":
			_, _ = w.Write([]byte(`{"results":[{"id":7,"klass":"RuntimeError","message":"boom"}]}`))
		case "/v2/projects/p1/faults/7/notices":
			_, _ = w.Write([]byte(`{"results":[{"backtrace":[{"file":"[PROJECT_ROOT]/a.go","method":"Foo","number":"12"},{"file":"b.go","method":"Bar","number":3}]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	t.Setenv(eghoneybadger.EnvProjectID, "p1")
	c := eghoneybadger.New().Token("secret").Endpoint(srv.URL).Limit(5)
	faults, err := c.Recent(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if len(faults) != 1 || faults[0].ID.String() != "7" || faults[0].Message != "boom" {
		t.Fatalf("unexpected faults: %+v", faults)
	}

	if user != "secret" || pass != "" {
		t.Fatalf("expected the token as the basic auth username, got %q:%q", user, pass)
	}

	if queries[0] != "limit=5&order=recent&q=-is%3Aresolved" {
		t.Fatalf("unexpected query: %s", queries[0])
	}

	n, err := c.Latest(context.Background(), faults[0])
	if err != nil {
		t.Fatal(err)
	}

	if n == nil || len(n.Backtrace) != 2 || n.Backtrace[0].Number != 12 || n.Backtrace[1].Number != 3 {
		t.Fatalf("unexpected notice: %+v", n)
	}

	if want := "Foo\n\t[PROJECT_ROOT]/a.go:12\nBar\n\tb.go:3\n"; n.String() != want {
		t.Fatalf("unexpected stacktrace: %q", n.String())
	}
}

func TestClientErrorStatus(t *testing.T) {
	t.Setenv(eghoneybadger.EnvProjectID, "p1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)

	if _, err := eghoneybadger.New().Endpoint(srv.URL).Recent(context.Background()); err == nil {
		t.Fatal("expected an error for a non 200 response")
	}
}

func TestClientRequiresProject(t *testing.T) {
	t.Setenv(eghoneybadger.EnvProjectID, "")
	if _, err := eghoneybadger.New().Recent(context.Background()); err == nil {
		t.Fatal("expected an error without a project")
	}
}
