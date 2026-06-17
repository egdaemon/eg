package egllm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/egdaemon/eg/internal/httpx"
)

type llm struct {
	c        *http.Client
	endpoint string
}

func New() llm {
	return llm{
		c:        &http.Client{},
		endpoint: "http://localhost:8080",
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

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model       string         `json:"model,omitempty"`
	Messages    []message      `json:"messages"`
	Stream      bool           `json:"stream"`
	Temperature float64        `json:"temperature"`
	Think       bool           `json:"think"`
	ExtraBody   map[string]any `json:"extra_body,omitempty"`
}

type response struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

// Generate sends prompt as a single user message to the llama.cpp server's
// OpenAI-compatible chat completions endpoint and returns the generated content.
func Generate(ctx context.Context, c Client, model string, prompt string) (string, error) {
	encoded, err := json.Marshal(request{
		Model:  model,
		Stream: false,
		Messages: []message{
			{Role: "user", Content: prompt},
		},
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
