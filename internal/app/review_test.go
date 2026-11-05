package app_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"codingtutor/internal/ai"
)

var reviewPath = regexp.MustCompile(`^/review/(\d+)$`)

const studentCode = "func Sum(x []int) int {\n\tfor _, v := range x {\n\t\ttotal := 0\n\t\ttotal += v\n\t}\n\treturn 0 // <b>not html</b>\n}"

func TestFreeFormCodeReview(t *testing.T) {
	fake := &fakeAI{}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Rev", "rev@example.com")

	r := c.post("/review", url.Values{"code": {studentCode}})
	if r.Status != http.StatusSeeOther || !reviewPath.MatchString(r.Location) {
		t.Fatalf("submit review: %d %q\n%s", r.Status, r.Location, r.Body)
	}
	if !strings.Contains(fake.lastUserPrompt(), "   3 | \t\ttotal := 0") {
		t.Error("code should be sent to the AI with line numbers")
	}

	page := c.get(r.Location)
	mustContain(t, page.Body,
		"Total is reset in the loop", "Declare total before the loop.", // actual error
		"Use a clearer name", // suggestion, shown separately
		"An empty slice",     // edge case
		"not compiled or executed",
		"&lt;b&gt;not html&lt;/b&gt;", // original code preserved and escaped
	)
	if strings.Contains(page.Body, "<b>not html</b>") {
		t.Fatal("submitted code must be escaped")
	}

	var stored string
	env.deps.DB.QueryRow(`SELECT code FROM exercise_submissions`).Scan(&stored)
	if stored != studentCode {
		t.Fatal("original submission must be stored unchanged")
	}
	mustContain(t, c.get("/review").Body, "Free-form review", "never compiled or executed")
}

func TestExerciseSubmissionUpdatesProgress(t *testing.T) {
	fake := &fakeAI{}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Sub", "sub@example.com")
	path := generateExercise(t, c)

	r := c.post(path+"/submissions", url.Values{"code": {studentCode}})
	if r.Status != http.StatusSeeOther {
		t.Fatalf("submit solution: %d\n%s", r.Status, r.Body)
	}
	if !strings.Contains(fake.lastUserPrompt(), "Sum a slice") {
		t.Error("exercise context should be sent with the review")
	}
	c.post(path+"/submissions", url.Values{"code": {studentCode}})

	var attempted, completed int
	env.deps.DB.QueryRow(`SELECT exercises_attempted, exercises_completed FROM learning_progress`).Scan(&attempted, &completed)
	if attempted != 1 || completed != 0 {
		t.Fatalf("attempted=%d completed=%d, want 1/0", attempted, completed)
	}
	page := c.get(path)
	mustContain(t, page.Body, "Your submissions", "Needs work")
	mustContain(t, c.get("/dashboard").Body, "Submitted a solution")
}

func TestCorrectSubmissionCompletesExercise(t *testing.T) {
	fake := &fakeAI{review: `{"summary": "Sums the slice.", "looks_correct": true, "errors": [], "edge_cases": [], "suggestions": []}`}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Ace", "ace@example.com")
	path := generateExercise(t, c)

	r := c.post(path+"/submissions", url.Values{"code": {"func Sum(n []int) int { t := 0; for _, v := range n { t += v }; return t }"}})
	if r.Status != http.StatusSeeOther {
		t.Fatalf("submit: %d", r.Status)
	}
	mustContain(t, c.get(r.Location).Body, "No errors found")
	mustContain(t, c.get(path).Body, `<span class="badge badge-success">Completed</span>`)
	var attempted, completed int
	env.deps.DB.QueryRow(`SELECT exercises_attempted, exercises_completed FROM learning_progress`).Scan(&attempted, &completed)
	if attempted != 1 || completed != 1 {
		t.Fatalf("attempted=%d completed=%d, want 1/1", attempted, completed)
	}
}

func TestReviewValidationErrorsAndPrivacy(t *testing.T) {
	fake := &fakeAI{}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Pat", "pat@example.com")

	if r := c.post("/review", url.Values{"code": {"   "}}); r.Status != http.StatusUnprocessableEntity {
		t.Errorf("empty code: %d", r.Status)
	}
	if r := c.post("/review", url.Values{"code": {strings.Repeat("x", 20001)}}); r.Status != http.StatusUnprocessableEntity {
		t.Errorf("oversized code: %d", r.Status)
	}

	fake.setFail(ai.ErrTimeout)
	r := c.post("/review", url.Values{"code": {"package main"}})
	if r.Status != http.StatusServiceUnavailable {
		t.Fatalf("AI timeout: %d", r.Status)
	}
	mustContain(t, r.Body, "took too long", "package main")
	fake.setFail(nil)

	r = c.post("/review", url.Values{"code": {"package main"}})
	other := env.newClient(t)
	other.register("Nosy", "nosy@example.com")
	if res := other.get(r.Location); res.Status != http.StatusNotFound {
		t.Errorf("other user's review: %d", res.Status)
	}
}
