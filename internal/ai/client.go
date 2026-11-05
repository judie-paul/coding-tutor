package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// Message is one chat message sent to the model.
type Message struct {
	Role    string `json:"role"` // system, user or assistant
	Content string `json:"content"`
}

// CompletionRequest is a provider-neutral chat completion request.
type CompletionRequest struct {
	Messages    []Message
	MaxTokens   int
	Temperature float64
	JSON        bool // ask the provider for a JSON object response
}

// Completer sends chat completion requests to a model. It is the only
// interface that touches the network, so tests replace it with a fake.
type Completer interface {
	Complete(ctx context.Context, req CompletionRequest) (string, error)
}

// CompleterFunc adapts a function to the Completer interface.
type CompleterFunc func(ctx context.Context, req CompletionRequest) (string, error)

// Complete calls f.
func (f CompleterFunc) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	return f(ctx, req)
}

// OpenAIClient calls any OpenAI-compatible /chat/completions endpoint
// (OpenAI, OpenRouter, Groq, Ollama, LM Studio, vLLM, ...).
type OpenAIClient struct {
	baseURL  string
	apiKey   string
	model    string
	jsonMode bool
	http     *http.Client
	logger   *slog.Logger
}

// ClientConfig configures an OpenAIClient.
type ClientConfig struct {
	BaseURL  string
	APIKey   string
	Model    string
	Timeout  time.Duration
	JSONMode bool
}

// NewOpenAIClient creates a client. The API key stays on the server.
func NewOpenAIClient(cfg ClientConfig, logger *slog.Logger) *OpenAIClient {
	return &OpenAIClient{
		baseURL:  strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:   cfg.APIKey,
		model:    cfg.Model,
		jsonMode: cfg.JSONMode,
		http:     &http.Client{Timeout: cfg.Timeout},
		logger:   logger,
	}
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	MaxTokens      int             `json:"max_tokens,omitempty"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete sends the request and returns the model's reply text.
func (c *OpenAIClient) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	if c.baseURL == "" || c.model == "" {
		return "", ErrNotConfigured
	}
	body := chatRequest{Model: c.model, Messages: req.Messages, MaxTokens: req.MaxTokens, Temperature: req.Temperature}
	if req.JSON && c.jsonMode {
		body.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("encode ai request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build ai request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	start := time.Now()
	resp, err := c.http.Do(httpReq)
	if err != nil {
		if isTimeout(ctx, err) {
			return "", ErrTimeout
		}
		c.logger.Error("ai request failed", "err", err)
		return "", fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		if isTimeout(ctx, err) {
			return "", ErrTimeout
		}
		return "", fmt.Errorf("%w: read body: %v", ErrUnavailable, err)
	}

	var parsed chatResponse
	jsonErr := json.Unmarshal(raw, &parsed)

	if resp.StatusCode != http.StatusOK {
		msg := ""
		if jsonErr == nil && parsed.Error != nil {
			msg = parsed.Error.Message
		}
		// Log details for operators; never return provider messages to users.
		c.logger.Error("ai provider error", "status", resp.StatusCode, "message", truncate(msg, 300))
		if resp.StatusCode == http.StatusTooManyRequests {
			return "", ErrProviderBusy
		}
		return "", fmt.Errorf("%w: provider returned status %d", ErrUnavailable, resp.StatusCode)
	}
	if jsonErr != nil || len(parsed.Choices) == 0 {
		return "", fmt.Errorf("%w: malformed provider response", ErrInvalidResponse)
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("%w: empty reply", ErrInvalidResponse)
	}
	c.logger.Debug("ai completion", "duration_ms", time.Since(start).Milliseconds(),
		"finish_reason", parsed.Choices[0].FinishReason, "chars", len(content))
	return content, nil
}

func isTimeout(ctx context.Context, err error) bool {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return true
	}
	var te interface{ Timeout() bool }
	return errors.As(err, &te) && te.Timeout()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
