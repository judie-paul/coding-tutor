// Package ai contains every interaction with the AI model: prompts,
// API calls and validation of structured responses. HTTP handlers and
// business logic never talk to the model directly.
package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// Errors returned by the AI layer. Use UserMessage to show them to students.
var (
	ErrNotConfigured   = errors.New("ai: not configured")
	ErrUnavailable     = errors.New("ai: service unavailable")
	ErrProviderBusy    = errors.New("ai: provider rate limit")
	ErrTimeout         = errors.New("ai: request timed out")
	ErrInvalidResponse = errors.New("ai: invalid response")
)

// UserMessage converts an AI error into a friendly, safe message.
func UserMessage(err error) string {
	switch {
	case errors.Is(err, ErrNotConfigured):
		return "The AI tutor is not configured yet. Ask the site administrator to set the AI settings."
	case errors.Is(err, ErrTimeout):
		return "The AI tutor took too long to respond. Please try again."
	case errors.Is(err, ErrProviderBusy):
		return "The AI tutor is busy right now. Please wait a moment and try again."
	case errors.Is(err, ErrInvalidResponse):
		return "The AI tutor returned an answer we couldn't understand. Please try again."
	default:
		return "The AI tutor is unavailable right now. Please try again in a moment."
	}
}

// IsAIError reports whether err came from the AI layer.
func IsAIError(err error) bool {
	return errors.Is(err, ErrNotConfigured) || errors.Is(err, ErrUnavailable) ||
		errors.Is(err, ErrProviderBusy) || errors.Is(err, ErrTimeout) || errors.Is(err, ErrInvalidResponse)
}

// Learner describes the student so explanations match their level.
type Learner struct {
	Language string // e.g. "Go"
	Level    string // beginner, intermediate or advanced
}

func (l Learner) language() string {
	if l.Language == "" {
		return "Go"
	}
	return l.Language
}

func (l Learner) level() string {
	if l.Level == "" {
		return "beginner"
	}
	return l.Level
}

// Tutor is the set of AI operations the application uses.
type Tutor interface {
	Explain(ctx context.Context, req ExplainRequest) (string, error)
	FollowUp(ctx context.Context, req FollowUpRequest) (string, error)
	GenerateExercise(ctx context.Context, req ExerciseRequest) (*Exercise, error)
	Hint(ctx context.Context, req HintRequest) (string, error)
	Solution(ctx context.Context, req HintRequest) (string, error)
	ReviewCode(ctx context.Context, req ReviewRequest) (*Review, error)
	GenerateQuiz(ctx context.Context, req QuizRequest) (*Quiz, error)
	Recommend(ctx context.Context, req RecommendRequest) (*Recommendations, error)
}

// Service implements Tutor on top of a Completer.
type Service struct {
	c      Completer
	logger *slog.Logger
}

var _ Tutor = (*Service)(nil)

// NewService returns a Service using c to reach the model.
func NewService(c Completer, logger *slog.Logger) *Service {
	return &Service{c: c, logger: logger}
}

// Token limits per operation keep costs predictable.
const (
	maxTokensChat     = 1400
	maxTokensExercise = 1800
	maxTokensHint     = 500
	maxTokensSolution = 1600
	maxTokensReview   = 1800
	maxTokensQuiz     = 2600
	maxTokensRecs     = 900
)

// Conversation context limits.
const (
	maxHistoryMessages = 16
	maxHistoryChars    = 16000
	maxQuestionChars   = 4000
)

// ---------------------------------------------------------------- Tutoring

// ExplainRequest asks for an explanation that starts a new conversation.
type ExplainRequest struct {
	Learner  Learner
	Topic    string // optional topic title for context
	Question string
}

// FollowUpRequest continues a conversation.
type FollowUpRequest struct {
	Learner  Learner
	Topic    string
	History  []Message // earlier user/assistant messages, oldest first
	Question string
}

// Explain answers a new question about a programming concept.
func (s *Service) Explain(ctx context.Context, req ExplainRequest) (string, error) {
	msgs := []Message{
		{Role: "system", Content: tutorSystemPrompt(req.Learner, req.Topic)},
		{Role: "user", Content: clip(req.Question, maxQuestionChars)},
	}
	return s.text(ctx, msgs, maxTokensChat, 0.5)
}

// FollowUp answers a question in the context of an existing conversation.
func (s *Service) FollowUp(ctx context.Context, req FollowUpRequest) (string, error) {
	msgs := []Message{{Role: "system", Content: tutorSystemPrompt(req.Learner, req.Topic)}}
	msgs = append(msgs, TrimHistory(req.History, maxHistoryMessages, maxHistoryChars)...)
	msgs = append(msgs, Message{Role: "user", Content: clip(req.Question, maxQuestionChars)})
	return s.text(ctx, msgs, maxTokensChat, 0.5)
}

// TrimHistory keeps the most recent messages within count and size limits,
// always starting with a user message so the model sees coherent turns.
func TrimHistory(history []Message, maxMessages, maxChars int) []Message {
	start := len(history)
	total := 0
	for i := len(history) - 1; i >= 0; i-- {
		n := len(history[i].Content)
		if len(history)-i > maxMessages || total+n > maxChars {
			break
		}
		total += n
		start = i
	}
	for start < len(history) && history[start].Role != "user" {
		start++
	}
	out := make([]Message, 0, len(history)-start)
	for _, m := range history[start:] {
		if m.Role == "user" || m.Role == "assistant" {
			out = append(out, m)
		}
	}
	return out
}

// ---------------------------------------------------------------- Hints

// HintRequest asks for the next progressive hint (or the full solution).
type HintRequest struct {
	Learner       Learner
	Exercise      *Exercise
	Level         int      // 1, 2 or 3
	PreviousHints []string // hints already shown, in order
	StudentCode   string   // latest attempt, may be empty
}

// Hint returns a progressive hint that does not reveal the full solution.
func (s *Service) Hint(ctx context.Context, req HintRequest) (string, error) {
	if req.Exercise == nil || req.Level < 1 || req.Level > 3 {
		return "", fmt.Errorf("invalid hint request")
	}
	msgs := []Message{
		{Role: "system", Content: hintSystemPrompt(req.Learner, req.Level)},
		{Role: "user", Content: hintUserPrompt(req)},
	}
	return s.text(ctx, msgs, maxTokensHint, 0.4)
}

// Solution returns a complete worked solution. Only call this when the
// student has explicitly asked for it.
func (s *Service) Solution(ctx context.Context, req HintRequest) (string, error) {
	if req.Exercise == nil {
		return "", fmt.Errorf("invalid solution request")
	}
	msgs := []Message{
		{Role: "system", Content: solutionSystemPrompt(req.Learner)},
		{Role: "user", Content: hintUserPrompt(req)},
	}
	return s.text(ctx, msgs, maxTokensSolution, 0.3)
}

// ---------------------------------------------------------------- Helpers

// text runs a free-text completion.
func (s *Service) text(ctx context.Context, msgs []Message, maxTokens int, temp float64) (string, error) {
	out, err := s.c.Complete(ctx, CompletionRequest{Messages: msgs, MaxTokens: maxTokens, Temperature: temp})
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", ErrInvalidResponse
	}
	return out, nil
}

// structured runs a JSON completion, decoding and validating the result
// with parse. Invalid responses are retried once before giving up.
func structured[T any](ctx context.Context, s *Service, op string, msgs []Message, maxTokens int, parse func(string) (T, error)) (T, error) {
	var zero T
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		out, err := s.c.Complete(ctx, CompletionRequest{Messages: msgs, MaxTokens: maxTokens, Temperature: 0.4, JSON: true})
		if err != nil {
			if errors.Is(err, ErrInvalidResponse) && attempt == 1 {
				lastErr = err
				continue
			}
			return zero, err
		}
		v, err := parse(out)
		if err == nil {
			return v, nil
		}
		lastErr = err
		s.logger.Warn("ai returned invalid structured response", "op", op, "attempt", attempt, "err", err)
	}
	if errors.Is(lastErr, ErrInvalidResponse) {
		return zero, lastErr
	}
	return zero, fmt.Errorf("%w: %v", ErrInvalidResponse, lastErr)
}

// clip shortens s to at most n bytes without splitting a UTF-8 character.
func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !isRuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
