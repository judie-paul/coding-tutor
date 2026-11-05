package app_test

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"codingtutor/internal/ai"
)

func TestProgressPageReflectsRecordedActivity(t *testing.T) {
	fake := &fakeAI{}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Pam", "pam@example.com")

	empty := c.get("/progress")
	if empty.Status != http.StatusOK {
		t.Fatalf("progress page: %d", empty.Status)
	}
	mustContain(t, empty.Body, "0%", "Nothing stands out yet", "Your learning activity will be recorded here")

	// Do real work: complete a lesson, submit an exercise twice without success, fail a quiz.
	c.post("/topics/slices-arrays/complete", url.Values{"completed": {"1"}})
	ex := generateExercise(t, c)
	c.post(ex+"/submissions", url.Values{"code": {"x"}})
	quiz := createQuiz(t, c)
	c.post(quiz+"/attempts", url.Values{}) // all unanswered -> 0%

	page := c.get("/progress")
	mustContain(t, page.Body,
		"Arrays and Slices",
		"Your last quiz score was below 60%.", // struggle
		"Slices check",                        // recent quiz
		"Completed the lesson",                // history
		"45%",                                 // 30 lesson + 15 attempted exercise
	)

	// AI study advice uses only the factual summary and validated topic names.
	r := c.post("/progress/recommendations", nil)
	if r.Status != http.StatusOK {
		t.Fatalf("recommendations: %d", r.Status)
	}
	mustContain(t, r.Body, "Keep going!", "Go Basics", "never changed by it")
	prompt := fake.lastUserPrompt()
	if !strings.Contains(prompt, "Arrays and Slices (beginner): mastery 45%") || !strings.Contains(prompt, "latest 0%") {
		t.Errorf("fact sheet missing real data:\n%s", prompt)
	}

	fake.setFail(ai.ErrUnavailable)
	r = c.post("/progress/recommendations", nil)
	if r.Status != http.StatusServiceUnavailable {
		t.Fatalf("recommendations with AI down: %d", r.Status)
	}
	mustContain(t, r.Body, "unavailable", "Topic by topic")
}

func TestProgressHistoryPagination(t *testing.T) {
	env := newTestEnv(t, withAI(&fakeAI{}))
	c := env.newClient(t)
	c.register("Hal", "hal@example.com")
	var uid int64
	env.deps.DB.QueryRow(`SELECT id FROM users`).Scan(&uid)
	for i := 0; i < 25; i++ {
		env.deps.DB.Exec(`INSERT INTO activity_logs (user_id, kind, description, created_at) VALUES (?, 'x', ?, ?)`,
			uid, fmt.Sprintf("Activity number %02d", i), fmt.Sprintf("2026-01-01T00:00:%02dZ", i))
	}
	first := c.get("/progress")
	mustContain(t, first.Body, "Activity number 24", "Older →")
	if strings.Contains(first.Body, "Activity number 04") {
		t.Error("first page should hold only the newest 20 entries")
	}
	second := c.get("/progress?page=2")
	mustContain(t, second.Body, "Activity number 04", "← Newer")
}
