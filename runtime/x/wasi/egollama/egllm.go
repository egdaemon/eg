package egollama

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
		endpoint: "http://localhost:11434",
	}
}

func (t llm) Do(r *http.Request) (*http.Response, error) {
	return t.c.Do(r)
}

type Client interface {
	Do(r *http.Request) (*http.Response, error)
	Host() string
}

func Generate(ctx context.Context, c Client, model string, prompt string) (string, error) {
	type Request struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Stream bool   `json:"stream"`
	}

	type Response struct {
		Model              string `json:"model"`
		Response           string `json:"response"`
		Done               bool   `json:"done"`
		LoadDuration       uint64 `json:"load_duration"`
		PromptEvalDuration uint64 `json:"prompt_eval_duration"`
		TotalDuration      uint64 `json:"total_duration"`
	}

	var (
		generated Response
	)
	encoded, err := json.Marshal(Request{
		Model:  model,
		Prompt: prompt,
		Stream: false,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/generate", c.Host()), bytes.NewReader(encoded))
	if err != nil {
		return "", err
	}

	resp, err := httpx.AsError(c.Do(req))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&generated); err != nil {
		return "", err
	}

	return generated.Response, nil
}
