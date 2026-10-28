package egllm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/runtime/wasi/eg"
)

const (
	// EnvAnthropicAPIKey is the standard variable holding the anthropic api key.
	EnvAnthropicAPIKey = "ANTHROPIC_API_KEY"
	// EnvAnthropicBaseURL is the standard variable overriding the anthropic api url.
	EnvAnthropicBaseURL = "ANTHROPIC_BASE_URL"

	// DefaultAnthropicEndpoint is the hosted anthropic api.
	DefaultAnthropicEndpoint = "https://api.anthropic.com"

	anthropicVersion   = "2023-06-01"
	claudeMaxTokens    = 8192 // default limit, the messages api requires an explicit one.
	claudeSystemPrompt = "system"
)

type claude struct {
	model    string
	key      string // empty defers to EnvAnthropicAPIKey.
	endpoint string // empty defers to EnvAnthropicBaseURL, then DefaultAnthropicEndpoint.
	tokens   int    // zero defers to claudeMaxTokens.
}

// Claude is a Judge backed by the anthropic messages api running model.
// there is nothing to manage so its Server implementation, With, is a pass-through.
func Claude(model string) claude {
	return claude{model: model}
}

// APIKey sets the api key, defaults to the value of EnvAnthropicAPIKey.
func (t claude) APIKey(v string) claude {
	t.key = v
	return t
}

// Endpoint sets the base url of the api, defaults to the value of EnvAnthropicBaseURL
// and then DefaultAnthropicEndpoint.
func (t claude) Endpoint(v string) claude {
	t.endpoint = strings.TrimRight(v, "/")
	return t
}

// MaxTokens sets the maximum number of tokens generated per response, defaults to 8192.
func (t claude) MaxTokens(v int) claude {
	t.tokens = v
	return t
}

// With runs ops in sequence. there is nothing to set up or tear down, it exists so
// the judge is interchangeable with those that are a Server.
func (t claude) With(ops ...eg.OpFn) eg.OpFn {
	if len(ops) == 1 {
		return ops[0]
	}

	return eg.Sequential(ops...)
}

type claudeRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system,omitempty"`
	Messages  []Message `json:"messages"`
}

type claudeResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}

// claudeErrorBodyLimit is the most of an error response read for diagnostics.
const claudeErrorBodyLimit = 4096

// ClaudeError is an error response from the anthropic api, it carries the reason the api gave,
// which the status alone doesn't, e.g.) a depleted credit balance is a 400.
// it unwraps to the httpx.Error of the response, so the status code can still be checked with httpx.IsStatusError.
type ClaudeError struct {
	Status  int    // http status code.
	Type    string // api error type, e.g.) invalid_request_error. empty when the body isn't an api error.
	Message string // api error message, or the raw body when it isn't an api error.
	cause   error
}

func newClaudeError(resp *http.Response, cause error) *ClaudeError {
	e := &ClaudeError{Status: resp.StatusCode, cause: cause}

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, claudeErrorBodyLimit))

	var body struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(raw, &body); err == nil && body.Error.Message != "" {
		e.Type, e.Message = body.Error.Type, body.Error.Message
	} else {
		e.Message = strings.TrimSpace(string(raw))
	}

	return e
}

func (t *ClaudeError) Error() string {
	var b strings.Builder
	b.WriteString("anthropic api: ")
	b.WriteString(t.cause.Error())

	if t.Type != "" {
		b.WriteString(": ")
		b.WriteString(t.Type)
	}

	if t.Message != "" {
		b.WriteString(": ")
		b.WriteString(t.Message)
	}

	return b.String()
}

func (t *ClaudeError) Unwrap() error {
	return t.cause
}

func (t claude) Do(ctx context.Context, m ...Message) (string, error) {
	key := t.key
	if key == "" {
		key = envx.String("", EnvAnthropicAPIKey)
	}
	if key == "" {
		return "", errors.New("anthropic api key required: set " + EnvAnthropicAPIKey)
	}

	endpoint := t.endpoint
	if endpoint == "" {
		endpoint = strings.TrimRight(envx.String(DefaultAnthropicEndpoint, EnvAnthropicBaseURL), "/")
	}

	// the messages api has no system role, it is a top level field.
	var system []string
	msgs := make([]Message, 0, len(m))
	for _, msg := range m {
		if msg.Role == claudeSystemPrompt {
			system = append(system, msg.Content)
			continue
		}
		msgs = append(msgs, msg)
	}

	tokens := t.tokens
	if tokens <= 0 {
		tokens = claudeMaxTokens
	}

	encoded, err := json.Marshal(claudeRequest{
		Model:     t.model,
		MaxTokens: tokens,
		System:    strings.Join(system, "\n\n"),
		Messages:  msgs,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/v1/messages", bytes.NewReader(encoded))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if cause := httpx.ErrorCode(resp); cause != nil {
		return "", newClaudeError(resp, cause)
	}

	var generated claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&generated); err != nil {
		return "", err
	}

	var b strings.Builder
	for _, c := range generated.Content {
		if c.Type == "text" {
			b.WriteString(c.Text)
		}
	}

	return b.String(), nil
}
