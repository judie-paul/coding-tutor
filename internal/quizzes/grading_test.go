package quizzes

import (
	"testing"

	"codingtutor/internal/ai"
)

func TestIsCorrect(t *testing.T) {
	mc := Question{Type: ai.MultipleChoice, Answer: "append", Options: []string{"append", "push"}}
	tf := Question{Type: ai.TrueFalse, Answer: "True"}
	sa := Question{Type: ai.ShortAnswer, Answer: "len", AcceptedAnswers: []string{"len()", "the len function"}}

	tests := []struct {
		q      Question
		answer string
		want   bool
	}{
		{mc, "append", true},
		{mc, "  Append ", true},
		{mc, "push", false},
		{mc, "", false},
		{tf, "true", true},
		{tf, "False", false},
		{sa, "len", true},
		{sa, "LEN.", true},
		{sa, "len()", true},
		{sa, "The  len function", true},
		{sa, "cap", false},
		{mc, "len()", false}, // accepted answers only apply to short answers
	}
	for _, tt := range tests {
		if got := IsCorrect(tt.q, tt.answer); got != tt.want {
			t.Errorf("IsCorrect(%s, %q) = %v, want %v", tt.q.Type, tt.answer, got, tt.want)
		}
	}
}

func TestGrade(t *testing.T) {
	qs := []Question{
		{ID: 1, Type: ai.TrueFalse, Answer: "True"},
		{ID: 2, Type: ai.TrueFalse, Answer: "False"},
		{ID: 3, Type: ai.ShortAnswer, Answer: "make"},
	}
	s := Grade(qs, map[int64]string{1: "True", 2: "True"}) // 3 unanswered
	if s.Correct != 1 || s.Total != 3 || s.Percent != 33 {
		t.Errorf("unexpected score: %+v", s)
	}
	if !s.Results[1] || s.Results[2] || s.Results[3] {
		t.Errorf("unexpected per-question results: %v", s.Results)
	}
	s = Grade(qs, map[int64]string{1: "true", 2: "false", 3: "make"})
	if s.Percent != 100 {
		t.Errorf("perfect score = %d", s.Percent)
	}
	s = Grade(qs[:2], map[int64]string{1: "True"})
	if s.Percent != 50 {
		t.Errorf("rounded percent = %d", s.Percent)
	}
	if Grade(nil, nil).Percent != 0 {
		t.Error("empty quiz should score 0")
	}
}
