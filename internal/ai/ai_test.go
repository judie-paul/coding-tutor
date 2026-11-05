package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

var discard = slog.New(slog.NewTextHandler(io.Discard, nil))

func TestOpenAIClientSendsRequestAndParsesReply(t *testing.T) {
	var got chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer secret-key" {
			t.Errorf("authorization = %q", auth)
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"choices":[{"message":{"content":"  Hello there  "},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	c := NewOpenAIClient(ClientConfig{BaseURL: srv.URL + "/v1/", APIKey: "secret-key", Model: "m1", Timeout: time.Second, JSONMode: true}, discard)
	out, err := c.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: "user", Content: "hi"}}, MaxTokens: 50, JSON: true,
	})
	if err != nil || out != "Hello there" {
		t.Fatalf("Complete = %q, %v", out, err)
	}
	if got.Model != "m1" || got.MaxTokens != 50 || got.ResponseFormat == nil || got.ResponseFormat.Type != "json_object" {
		t.Errorf("unexpected request body: %+v", got)
	}
}

func TestOpenAIClientErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"server error", 500, `{"error":{"message":"boom, key sk-123"}}`, ErrUnavailable},
		{"unauthorized", 401, `{"error":{"message":"bad key"}}`, ErrUnavailable},
		{"rate limited", 429, `{}`, ErrProviderBusy},
		{"no choices", 200, `{"choices":[]}`, ErrInvalidResponse},
		{"not json", 200, `<html>`, ErrInvalidResponse},
		{"empty content", 200, `{"choices":[{"message":{"content":"   "}}]}`, ErrInvalidResponse},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()
			c := NewOpenAIClient(ClientConfig{BaseURL: srv.URL, Model: "m", Timeout: time.Second}, discard)
			_, err := c.Complete(context.Background(), CompletionRequest{})
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
			if strings.Contains(UserMessage(err), "sk-123") {
				t.Fatal("provider details leaked into user message")
			}
		})
	}
}

func TestOpenAIClientTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()
	c := NewOpenAIClient(ClientConfig{BaseURL: srv.URL, Model: "m", Timeout: 50 * time.Millisecond}, discard)
	if _, err := c.Complete(context.Background(), CompletionRequest{}); !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
}

func TestOpenAIClientNotConfigured(t *testing.T) {
	c := NewOpenAIClient(ClientConfig{}, discard)
	if _, err := c.Complete(context.Background(), CompletionRequest{}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured, got %v", err)
	}
}

func TestTrimHistory(t *testing.T) {
	var h []Message
	for i := 0; i < 10; i++ {
		h = append(h, Message{Role: "user", Content: "question"}, Message{Role: "assistant", Content: "answer"})
	}
	got := TrimHistory(h, 5, 100000)
	if len(got) != 4 || got[0].Role != "user" {
		t.Fatalf("expected 4 messages starting with user, got %d (%v)", len(got), got)
	}
	got = TrimHistory(h, 100, 30) // ~3 messages by size, then aligned to a user turn
	if len(got) == 0 || got[0].Role != "user" {
		t.Fatalf("size trimming broke turn alignment: %v", got)
	}
	total := 0
	for _, m := range got {
		total += len(m.Content)
	}
	if total > 30 {
		t.Fatalf("history exceeds size limit: %d", total)
	}
}

func TestParseExercise(t *testing.T) {
	raw := "Here you go:\n```json\n" + `{
		"title": " Sum a slice ",
		"description": "Write Sum.",
		"learning_objective": "Practice loops.",
		"requirements": ["Return the total", "  "],
		"examples": [{"input": "[]int{1,2}", "output": "3"}, {"input": "", "output": ""}],
		"constraints": [],
		"hints": ["a", "b", "c", "d"],
		"concepts": ["for loops"],
		"starter_code": "func Sum(n []int) int {\n}\n"
	}` + "\n```"
	ex, err := ParseExercise(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ex.Title != "Sum a slice" || len(ex.Requirements) != 1 || len(ex.Examples) != 1 || len(ex.Hints) != 3 {
		t.Errorf("unexpected exercise: %+v", ex)
	}

	for _, bad := range []string{
		`not json`,
		`{"title": "", "description": "d", "learning_objective": "o", "requirements": ["r"]}`,
		`{"title": "t", "description": "d", "learning_objective": "o", "requirements": []}`,
	} {
		if _, err := ParseExercise(bad); !errors.Is(err, ErrInvalidResponse) {
			t.Errorf("ParseExercise(%q) should fail with ErrInvalidResponse, got %v", bad, err)
		}
	}
}

func TestParseReview(t *testing.T) {
	r, err := ParseReview(`{
		"summary": "Adds numbers.",
		"looks_correct": true,
		"errors": [{"kind": "SYNTAX", "line": 3, "title": "Missing brace", "explanation": "x"}, {"kind": "weird", "title": "", "explanation": "y"}, {}],
		"edge_cases": ["empty slice"],
		"suggestions": [{"kind": "style", "title": "Name", "explanation": "z", "line": -4}]
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if r.LooksCorrect {
		t.Error("a review with errors must not be marked correct")
	}
	if len(r.Errors) != 2 || r.Errors[0].Kind != "syntax" || r.Errors[1].Kind != "logic" || r.Errors[1].Title == "" {
		t.Errorf("errors not normalized: %+v", r.Errors)
	}
	if r.Suggestions[0].Line != 0 {
		t.Error("invalid line numbers should be cleared")
	}
	if _, err := ParseReview(`{"summary": ""}`); err == nil {
		t.Error("empty summary should be rejected")
	}
}

func TestParseQuiz(t *testing.T) {
	raw := `{"title": "Maps", "questions": [
		{"type": "multiple_choice", "prompt": "Q1", "options": ["a", "b", "c", "b"], "answer": "B", "explanation": "e"},
		{"type": "true_false", "prompt": "Q2", "answer": "false", "explanation": "e"},
		{"type": "short_answer", "prompt": "Q3", "answer": "make", "accepted_answers": ["make()"], "explanation": "e"},
		{"type": "multiple_choice", "prompt": "bad: answer not an option", "options": ["x", "y"], "answer": "z", "explanation": "e"},
		{"type": "essay", "prompt": "unsupported", "answer": "x", "explanation": "e"},
		{"type": "true_false", "prompt": "no explanation", "answer": "true"}
	]}`
	q, err := ParseQuiz(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Questions) != 3 {
		t.Fatalf("expected 3 valid questions, got %d", len(q.Questions))
	}
	mc := q.Questions[0]
	if len(mc.Options) != 3 || mc.Answer != "b" {
		t.Errorf("multiple choice not normalized: %+v", mc)
	}
	if q.Questions[1].Answer != "False" || len(q.Questions[1].Options) != 2 {
		t.Errorf("true/false not normalized: %+v", q.Questions[1])
	}

	if _, err := ParseQuiz(`{"questions": [{"type": "true_false", "prompt": "Q", "answer": "true", "explanation": "e"}]}`); err == nil {
		t.Error("a quiz with fewer than 3 valid questions should be rejected")
	}
}

func TestParseRecommendationsKeepsOnlyKnownTopics(t *testing.T) {
	r, err := ParseRecommendations(`{"summary": "Good work.", "recommendations": [
		{"topic": "maps", "reason": "Low quiz score", "action": "Retake quiz"},
		{"topic": "Quantum Computing", "reason": "made up", "action": "x"}
	]}`, []string{"Maps", "Structs"})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != 1 || r.Items[0].Topic != "Maps" {
		t.Errorf("unexpected items: %+v", r.Items)
	}
}

func TestStructuredRetriesOnceThenFails(t *testing.T) {
	calls := 0
	fake := CompleterFunc(func(ctx context.Context, req CompletionRequest) (string, error) {
		calls++
		if !req.JSON {
			t.Error("structured requests must ask for JSON")
		}
		if calls == 1 {
			return "sorry, no json", nil
		}
		return `{"title":"T","description":"D","learning_objective":"O","requirements":["R"]}`, nil
	})
	svc := NewService(fake, discard)
	ex, err := svc.GenerateExercise(context.Background(), ExerciseRequest{Topic: "Maps", Difficulty: "beginner", Type: "function"})
	if err != nil || ex.Title != "T" || calls != 2 {
		t.Fatalf("expected success on retry: ex=%v err=%v calls=%d", ex, err, calls)
	}

	calls = 0
	bad := CompleterFunc(func(context.Context, CompletionRequest) (string, error) { calls++; return "{}", nil })
	if _, err := NewService(bad, discard).GenerateQuiz(context.Background(), QuizRequest{Count: 5}); !errors.Is(err, ErrInvalidResponse) || calls != 2 {
		t.Fatalf("expected ErrInvalidResponse after 2 calls, got %v after %d", err, calls)
	}
}

func TestPromptsCarryContext(t *testing.T) {
	var captured []Message
	fake := CompleterFunc(func(ctx context.Context, req CompletionRequest) (string, error) {
		captured = req.Messages
		return "ok", nil
	})
	svc := NewService(fake, discard)
	_, err := svc.FollowUp(context.Background(), FollowUpRequest{
		Learner:  Learner{Language: "Go", Level: "advanced"},
		Topic:    "Channels",
		History:  []Message{{Role: "user", Content: "What is a channel?"}, {Role: "assistant", Content: "A pipe."}},
		Question: "And buffered ones?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(captured) != 4 || captured[0].Role != "system" || captured[3].Content != "And buffered ones?" {
		t.Fatalf("unexpected messages: %+v", captured)
	}
	sys := captured[0].Content
	if !strings.Contains(sys, "Channels") || !strings.Contains(sys, "advanced") {
		t.Errorf("system prompt missing context: %s", sys)
	}

	svc.ReviewCode(context.Background(), ReviewRequest{Code: "package main\nfunc main() {}"})
	if !strings.Contains(captured[0].Content, "NOT been compiled") || !strings.Contains(captured[1].Content, "   2 | func main() {}") {
		t.Error("review prompt must state code is not executed and number lines")
	}

	if _, err := svc.Hint(context.Background(), HintRequest{Level: 4, Exercise: &Exercise{}}); err == nil {
		t.Error("hint level outside 1-3 must be rejected")
	}
}
