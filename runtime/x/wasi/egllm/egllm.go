package egllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/httpx"
)

// DefaultEndpoint is where llama.cpp's llama-server listens by default.
const DefaultEndpoint = "http://localhost:8080"

// Endpoint returns the base url of the llm server: the value of eg.EnvComputeLLMEndpoint, otherwise llama.cpp's default.
func Endpoint() string {
	return strings.TrimRight(envx.String(DefaultEndpoint, _eg.EnvComputeLLMEndpoint), "/")
}

// overridden reports whether the endpoint has been overridden, in which case the server isn't ours to manage.
func overridden() bool {
	return Endpoint() != DefaultEndpoint
}

type llm struct {
	c        *http.Client
	endpoint string
}

func New() llm {
	return llm{
		c:        &http.Client{},
		endpoint: Endpoint(),
	}
}

func (t llm) Host() string {
	return t.endpoint
}

func (t llm) Do(r *http.Request) (*http.Response, error) {
	return t.c.Do(r)
}

type Client interface {
	Do(r *http.Request) (*http.Response, error)
	Host() string
}

// Message is a single turn of a chat, role is one of "system", "user" or "assistant".
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model       string    `json:"model,omitempty"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature,omitempty"` // zero defers to the server/model default.
}

type response struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// Generate sends prompt as a single user message to the llama.cpp server's
// OpenAI-compatible chat completions endpoint and returns the generated content.
func Generate(ctx context.Context, c Client, model string, prompt string) (string, error) {
	return Chat(ctx, c, model, Message{Role: "user", Content: prompt})
}

// Chat sends the messages, in order, to the llama.cpp server's OpenAI-compatible
// chat completions endpoint and returns the generated content.
func Chat(ctx context.Context, c Client, model string, messages ...Message) (string, error) {
	encoded, err := json.Marshal(request{
		Model:    model,
		Stream:   false,
		Messages: messages,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/v1/chat/completions", c.Host()), bytes.NewReader(encoded))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpx.AsError(c.Do(req))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var generated response
	if err := json.NewDecoder(resp.Body).Decode(&generated); err != nil {
		return "", err
	}

	if len(generated.Choices) == 0 {
		return "", nil
	}

	return generated.Choices[0].Message.Content, nil
}
