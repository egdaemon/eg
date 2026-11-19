package apt

import (
	"bufio"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/egdaemon/eg/internal/httpx"
)

// Message of the apt method protocol: a status line followed by fields, terminated by a blank line.
// see /usr/share/doc/apt-doc/method.txt
type Message struct {
	Code   int
	Text   string
	Fields []Field
}

// Field of a message, fields are ordered and may repeat (Config-Item).
type Field struct {
	Key   string
	Value string
}

// Get the value of the first field with the key, blank when missing.
func (t Message) Get(key string) string {
	for _, f := range t.Fields {
		if strings.EqualFold(f.Key, key) {
			return f.Value
		}
	}

	return ""
}

func (t Message) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d %s\n", t.Code, t.Text)
	for _, f := range t.Fields {
		fmt.Fprintf(&b, "%s: %s\n", f.Key, f.Value)
	}
	b.WriteString("\n")
	return b.String()
}

// ReadMessage reads the next message, io.EOF when the input ends between messages.
func ReadMessage(r *bufio.Reader) (zero Message, err error) {
	var (
		m       Message
		started bool
	)

	for {
		line, rerr := r.ReadString('\n')
		line = strings.TrimRight(line, "\r\n")

		switch {
		case line == "" && started:
			return m, nil
		case line == "":
			// blank lines between messages are ignored.
			if rerr != nil {
				return zero, rerr
			}
			continue
		case !started:
			code, text, _ := strings.Cut(line, " ")
			if m.Code, err = strconv.Atoi(code); err != nil {
				return zero, fmt.Errorf("invalid message status line '%s'", line)
			}
			m.Text = text
			started = true
		default:
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				return zero, fmt.Errorf("invalid message field '%s'", line)
			}
			m.Fields = append(m.Fields, Field{Key: k, Value: strings.TrimSpace(v)})
		}

		if errors.Is(rerr, io.EOF) {
			// the input ended without the terminating blank line.
			return m, nil
		} else if rerr != nil {
			return zero, rerr
		}
	}
}

// Method is the apt acquire method for the egs scheme. apt runs /usr/lib/apt/methods/egs, which
// execs `eg apt transport`. an egs uri is fetched over https with a bearer token for the account
// in the host, <account id>.<domain>.
type Method struct {
	// Client returns the http client authorized for the account. it is called once per account.
	Client  func(account string) (*http.Client, error)
	clients map[string]*http.Client
}

func (t *Method) client(account string) (*http.Client, error) {
	if c, ok := t.clients[account]; ok {
		return c, nil
	}

	c, err := t.Client(account)
	if err != nil {
		return nil, err
	}

	if t.clients == nil {
		t.clients = map[string]*http.Client{}
	}
	t.clients[account] = c
	return c, nil
}

// Run speaks the method protocol until the input ends. acquires are processed in the order
// apt sends them, which keeps pipelined requests in order.
func (t *Method) Run(ctx context.Context, in io.Reader, out io.Writer) error {
	w := bufio.NewWriter(out)
	send := func(m Message) error {
		if _, err := w.WriteString(m.String()); err != nil {
			return err
		}
		return w.Flush()
	}

	if err := send(Message{Code: 100, Text: "Capabilities", Fields: []Field{
		{Key: "Version", Value: "1.2"},
		{Key: "Pipeline", Value: "true"},
		{Key: "Send-Config", Value: "true"},
	}}); err != nil {
		return err
	}

	r := bufio.NewReader(in)
	for {
		m, err := ReadMessage(r)
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			_ = send(Message{Code: 401, Text: "General Failure", Fields: []Field{{Key: "Message", Value: err.Error()}}})
			return err
		}

		switch m.Code {
		case 600:
			for _, reply := range t.acquire(ctx, m) {
				if err := send(reply); err != nil {
					return err
				}
			}
		default:
			// 601 Configuration and anything else apt sends is not needed.
		}
	}
}

// failure of an acquire, transient failures are retried by apt.
func failure(uri string, msg string, reason string, transient bool) []Message {
	fields := []Field{{Key: "URI", Value: uri}, {Key: "Message", Value: msg}}
	if reason != "" {
		fields = append(fields, Field{Key: "FailReason", Value: reason})
	}
	if transient {
		fields = append(fields, Field{Key: "Transient-Failure", Value: "true"})
	}
	return []Message{{Code: 400, Text: "URI Failure", Fields: fields}}
}

func (t *Method) acquire(ctx context.Context, m Message) []Message {
	var (
		uri      = m.Get("URI")
		filename = m.Get("Filename")
		ims      = m.Get("Last-Modified")
	)

	u, err := url.Parse(uri)
	if err != nil || u.Scheme != Scheme || u.Host == "" {
		return failure(uri, fmt.Sprintf("invalid uri, expected %s://<account>.<domain>/<repository>/...", Scheme), "", false)
	}

	if filename == "" {
		return failure(uri, "missing Filename", "", false)
	}

	account, _, _ := strings.Cut(u.Hostname(), ".")
	c, err := t.client(account)
	if err != nil {
		return failure(uri, fmt.Sprintf("unable to authorize account %s: %v", account, err), "", false)
	}

	target := *u
	target.Scheme = "https"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return failure(uri, err.Error(), "", false)
	}

	if ims != "" {
		req.Header.Set("If-Modified-Since", ims)
	}

	resp, err := c.Do(req)
	if err != nil {
		return failure(uri, err.Error(), "", true)
	}
	defer httpx.AutoClose(resp)

	switch {
	case resp.StatusCode == http.StatusNotModified:
		fields := []Field{{Key: "URI", Value: uri}, {Key: "Filename", Value: filename}, {Key: "IMS-Hit", Value: "true"}}
		if ims != "" {
			fields = append(fields, Field{Key: "Last-Modified", Value: ims})
		}
		return []Message{{Code: 201, Text: "URI Done", Fields: fields}}
	case resp.StatusCode == http.StatusOK:
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		// authorization failures are permanent, apt must not retry them.
		return failure(uri, fmt.Sprintf("%s, the eg identity of this machine is not authorized for the repository", resp.Status), fmt.Sprintf("HttpError%d", resp.StatusCode), false)
	default:
		return failure(uri, resp.Status, fmt.Sprintf("HttpError%d", resp.StatusCode), resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests)
	}

	lastmod := resp.Header.Get("Last-Modified")
	start := []Field{{Key: "URI", Value: uri}}
	if resp.ContentLength >= 0 {
		start = append(start, Field{Key: "Size", Value: strconv.FormatInt(resp.ContentLength, 10)})
	}
	if lastmod != "" {
		start = append(start, Field{Key: "Last-Modified", Value: lastmod})
	}
	started := Message{Code: 200, Text: "URI Start", Fields: start}

	done, err := download(resp.Body, filename)
	if err != nil {
		return append([]Message{started}, failure(uri, err.Error(), "", true)...)
	}

	if lastmod != "" {
		if ts, err := http.ParseTime(lastmod); err == nil {
			_ = os.Chtimes(filename, ts, ts)
		}
	}

	fields := []Field{
		{Key: "URI", Value: uri},
		{Key: "Filename", Value: filename},
		{Key: "Size", Value: strconv.FormatInt(done.size, 10)},
	}
	if lastmod != "" {
		fields = append(fields, Field{Key: "Last-Modified", Value: lastmod})
	}
	fields = append(fields, done.fields...)

	return []Message{started, {Code: 201, Text: "URI Done", Fields: fields}}
}

type downloaded struct {
	size   int64
	fields []Field
}

// download writes the body to the file, hashing it as it streams.
func download(body io.Reader, filename string) (zero downloaded, err error) {
	type named struct {
		key string
		h   hash.Hash
	}

	hashes := []named{
		{key: "MD5Sum-Hash", h: md5.New()},
		{key: "SHA1-Hash", h: sha1.New()},
		{key: "SHA256-Hash", h: sha256.New()},
		{key: "SHA512-Hash", h: sha512.New()},
	}

	writers := make([]io.Writer, 0, len(hashes)+1)
	for _, n := range hashes {
		writers = append(writers, n.h)
	}

	dst, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return zero, err
	}
	defer dst.Close()

	n, err := io.Copy(io.MultiWriter(append(writers, dst)...), body)
	if err != nil {
		return zero, fmt.Errorf("download interrupted after %d bytes: %w", n, err)
	}

	if err = dst.Close(); err != nil {
		return zero, err
	}

	out := downloaded{size: n}
	for _, n := range hashes {
		out.fields = append(out.fields, Field{Key: n.key, Value: hex.EncodeToString(n.h.Sum(nil))})
	}

	return out, nil
}

// SameHostRedirects refuses redirects to another host, the authorized client attaches the
// bearer token to every request it makes, including redirects.
func SameHostRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}

	if req.URL.Host != via[0].URL.Host {
		return fmt.Errorf("refusing redirect from %s to %s", via[0].URL.Host, req.URL.Host)
	}

	return nil
}
