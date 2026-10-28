// Package eghoneybadger is a client for the honeybadger data api, used to read the errors
// reported to a project.
package eghoneybadger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	// EnvAuthToken is the standard variable holding the personal auth token.
	EnvAuthToken = "HONEYBADGER_AUTH_TOKEN"
	// EnvProjectID is the standard variable holding the project to read.
	EnvProjectID = "HONEYBADGER_PROJECT_ID"
	// EnvBaseURL is the standard variable overriding the data api url.
	EnvBaseURL = "HONEYBADGER_BASE_URL"

	// DefaultEndpoint is the hosted data api.
	DefaultEndpoint = "https://app.honeybadger.io"
)

// Fault is an error reported to honeybadger, the notices of the same error are grouped under it.
type Fault struct {
	ID      json.Number `json:"id"`
	Klass   string      `json:"klass"`
	Message string      `json:"message"`
}

// Frame is a single line of a backtrace.
type Frame struct {
	File   string `json:"file"`
	Method string `json:"method"`
	Number Line   `json:"number"`
}

// Line is a line number, honeybadger reports it as either a number or a string.
type Line int

func (t *Line) UnmarshalJSON(b []byte) error {
	n, err := strconv.Atoi(strings.Trim(string(b), `"`))
	if err != nil {
		// a missing or non numeric line is not worth failing the backtrace.
		*t = 0
		return nil
	}

	*t = Line(n)
	return nil
}

// Notice is a single occurrence of a fault.
type Notice struct {
	Backtrace []Frame `json:"backtrace"`
}

// String renders the backtrace one frame per line, innermost first.
func (t Notice) String() string {
	var b strings.Builder
	for _, f := range t.Backtrace {
		fmt.Fprintf(&b, "%s\n\t%s:%d\n", f.Method, f.File, f.Number)
	}
	return b.String()
}

type client struct {
	token    string // empty defers to EnvAuthToken.
	endpoint string // empty defers to EnvBaseURL, then DefaultEndpoint.
	query    string
	limit    int // zero defers to the api default.
	http     *http.Client
}

// New is a client reading the faults of the project given by EnvProjectID.
// it only reads unresolved faults unless Query says otherwise.
func New() client {
	return client{query: "-is:resolved"}
}

// Token sets the personal auth token, defaults to the value of EnvAuthToken.
func (t client) Token(v string) client {
	t.token = v
	return t
}

// Endpoint sets the base url of the api, defaults to the value of EnvBaseURL
// and then DefaultEndpoint.
func (t client) Endpoint(v string) client {
	t.endpoint = strings.TrimRight(v, "/")
	return t
}

// Query sets the search faults must match, in the syntax of the honeybadger ui, defaults to -is:resolved.
func (t client) Query(v string) client {
	t.query = v
	return t
}

// Limit sets the maximum number of faults read, defaults to the api's.
func (t client) Limit(v int) client {
	t.limit = v
	return t
}

// HTTP sets the client used for requests, defaults to http.DefaultClient.
func (t client) HTTP(c *http.Client) client {
	t.http = c
	return t
}

// Recent are the faults that occurred most recently first.
func (t client) Recent(ctx context.Context) ([]Fault, error) {
	q := url.Values{"order": {"recent"}, "q": {t.query}}
	if t.limit > 0 {
		q.Set("limit", strconv.Itoa(t.limit))
	}

	var page struct {
		Results []Fault `json:"results"`
	}

	if err := t.get(ctx, "faults", q, &page); err != nil {
		return nil, err
	}

	return page.Results, nil
}

// Latest is the most recent notice of the fault, nil when it has none.
func (t client) Latest(ctx context.Context, fault Fault) (*Notice, error) {
	var page struct {
		Results []Notice `json:"results"`
	}

	if err := t.get(ctx, "faults/"+url.PathEscape(fault.ID.String())+"/notices", url.Values{"limit": {"1"}}, &page); err != nil {
		return nil, err
	}

	if len(page.Results) == 0 {
		return nil, nil
	}

	return &page.Results[0], nil
}

func (t client) get(ctx context.Context, path string, q url.Values, dst any) error {
	project := os.Getenv(EnvProjectID)
	if project == "" {
		return fmt.Errorf("honeybadger project is required, set %s", EnvProjectID)
	}

	endpoint := firstnonempty(t.endpoint, strings.TrimRight(os.Getenv(EnvBaseURL), "/"), DefaultEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/v2/projects/"+url.PathEscape(project)+"/"+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}

	// the personal auth token is the username, the password is empty.
	req.SetBasicAuth(firstnonempty(t.token, os.Getenv(EnvAuthToken)), "")
	req.Header.Set("Accept", "application/json")

	c := t.http
	if c == nil {
		c = http.DefaultClient
	}

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("honeybadger %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}

	return json.NewDecoder(resp.Body).Decode(dst)
}

func firstnonempty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}

	return ""
}
