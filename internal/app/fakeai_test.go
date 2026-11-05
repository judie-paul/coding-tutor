package app_test

import (
	"context"
	"strings"
	"sync"

	"codingtutor/internal/ai"
)

// fakeAI answers each AI operation with canned content, so HTTP tests run
// without network access or API costs. Set fail to simulate an outage.
type fakeAI struct {
	mu     sync.Mutex
	fail   error
	review string // optional override for code review responses
	calls  []ai.CompletionRequest
}

func (f *fakeAI) setFail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail = err
}

func (f *fakeAI) lastUserPrompt() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return ""
	}
	msgs := f.calls[len(f.calls)-1].Messages
	return msgs[len(msgs)-1].Content
}

func (f *fakeAI) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeAI) Complete(ctx context.Context, req ai.CompletionRequest) (string, error) {
	f.mu.Lock()
	f.calls = append(f.calls, req)
	fail := f.fail
	f.mu.Unlock()
	if fail != nil {
		return "", fail
	}
	sys := req.Messages[0].Content
	switch {
	case strings.Contains(sys, "design programming exercises"):
		return fakeExercise, nil
	case strings.Contains(sys, "giving hint"):
		return "Think about using a **for** loop.", nil
	case strings.Contains(sys, "complete solution"):
		return "```go\nfunc Sum(n []int) int { t := 0; for _, v := range n { t += v }; return t }\n```", nil
	case strings.Contains(sys, "code reviewer"):
		if f.review != "" {
			return f.review, nil
		}
		return fakeReview, nil
	case strings.Contains(sys, "programming quizzes"):
		return fakeQuiz, nil
	case strings.Contains(sys, "study coach"):
		return `{"summary": "Keep going!", "recommendations": [{"topic": "Go Basics", "reason": "You have not started it.", "action": "Read the lesson."}]}`, nil
	default:
		return "Goroutines are lightweight threads.\n\n```go\ngo work()\n```\n\n<script>alert(1)</script>", nil
	}
}

const fakeExercise = `{
  "title": "Sum a slice",
  "description": "Write a function Sum that returns the total of a slice of ints.",
  "learning_objective": "Practice for loops over slices.",
  "requirements": ["Define func Sum(nums []int) int", "Return 0 for an empty slice"],
  "examples": [{"input": "Sum([]int{1, 2, 3})", "output": "6"}],
  "constraints": ["Do not use any packages"],
  "hints": ["Start with a total of zero.", "Loop with range and add each value.", "for _, v := range nums { total += v }"],
  "concepts": ["for range", "slices"],
  "starter_code": "func Sum(nums []int) int {\n\treturn 0\n}"
}`

const fakeReview = `{
  "summary": "The function loops over the slice and adds the values.",
  "looks_correct": false,
  "errors": [{"kind": "logic", "line": 3, "title": "Total is reset in the loop", "explanation": "total := 0 inside the loop resets it.", "fix_hint": "Declare total before the loop."}],
  "edge_cases": ["An empty slice"],
  "suggestions": [{"kind": "naming", "line": 1, "title": "Use a clearer name", "explanation": "x is vague.", "fix_hint": "Rename x to nums."}],
  "corrected_example": "",
  "encouragement": "You're very close!"
}`

const fakeQuiz = `{
  "title": "Slices check",
  "questions": [
    {"type": "multiple_choice", "prompt": "Which builtin adds to a slice?", "options": ["append", "push", "add", "insert"], "answer": "append", "explanation": "append returns a new slice.", "wrong_explanations": {"push": "Go has no push."}},
    {"type": "true_false", "prompt": "Arrays have a fixed length.", "options": ["True", "False"], "answer": "True", "explanation": "Length is part of the type."},
    {"type": "short_answer", "prompt": "Which builtin returns a slice's length?", "answer": "len", "accepted_answers": ["len()"], "explanation": "len works on slices."}
  ]
}`
