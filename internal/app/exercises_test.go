package app_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"codingtutor/internal/ai"
)

var exercisePath = regexp.MustCompile(`^/exercises/(\d+)$`)

func generateExercise(t *testing.T, c *client) string {
	t.Helper()
	r := c.post("/exercises", url.Values{"topic_id": {"5"}, "difficulty": {"beginner"}, "type": {"function"}})
	if r.Status != http.StatusSeeOther || !exercisePath.MatchString(r.Location) {
		t.Fatalf("generate exercise: %d %q\n%s", r.Status, r.Location, r.Body)
	}
	return r.Location
}

func TestTopicsAndLessonCompletion(t *testing.T) {
	env := newTestEnv(t, withAI(&fakeAI{}))
	c := env.newClient(t)
	c.register("Tim", "tim@example.com")

	list := c.get("/topics")
	mustContain(t, list.Body, "Go Basics", "Goroutines", "Context and Cancellation")

	show := c.get("/topics/maps")
	if show.Status != http.StatusOK {
		t.Fatalf("topic page: %d", show.Status)
	}
	mustContain(t, show.Body, "The comma-ok idiom", "Mark lesson as completed", "/exercises/new?topic=maps")
	if r := c.get("/topics/no-such-topic"); r.Status != http.StatusNotFound {
		t.Errorf("unknown topic: %d", r.Status)
	}

	r := c.post("/topics/maps/complete", url.Values{"completed": {"1"}})
	if r.Status != http.StatusSeeOther {
		t.Fatalf("complete lesson: %d", r.Status)
	}
	show = c.get("/topics/maps")
	mustContain(t, show.Body, "Lesson completed · undo", "<strong>30%</strong>")
	dash := c.get("/dashboard")
	mustContain(t, dash.Body, "Completed the lesson")

	c.post("/topics/maps/complete", url.Values{"completed": {"0"}})
	if show = c.get("/topics/maps"); !strings.Contains(show.Body, "Mark lesson as completed") {
		t.Error("lesson should be unmarked")
	}
}

func TestExerciseGenerationAndCompletion(t *testing.T) {
	fake := &fakeAI{}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Eve", "eve@example.com")

	if r := c.get("/exercises/new?topic=slices-arrays"); r.Status != http.StatusOK {
		t.Fatalf("generator page: %d", r.Status)
	} else {
		mustContain(t, r.Body, "Generate an exercise", "Fix the bug")
	}

	path := generateExercise(t, c)
	if !strings.Contains(fake.lastUserPrompt(), "Arrays and Slices") {
		t.Errorf("prompt missing topic: %s", fake.lastUserPrompt())
	}
	page := c.get(path)
	mustContain(t, page.Body, "Sum a slice", "Practice for loops over slices.", "Return 0 for an empty slice", "Sum([]int{1, 2, 3})", "for range")
	// Pre-generated hints must not be revealed on the page.
	if strings.Contains(page.Body, "Start with a total of zero.") {
		t.Fatal("hints leaked onto the exercise page")
	}

	list := c.get("/exercises")
	mustContain(t, list.Body, "Sum a slice", "1 exercise, 0 completed")

	if r := c.post(path+"/complete", nil); r.Status != http.StatusSeeOther {
		t.Fatalf("complete: %d", r.Status)
	}
	c.post(path+"/complete", nil) // idempotent
	var completed int
	env.deps.DB.QueryRow(`SELECT exercises_completed FROM learning_progress`).Scan(&completed)
	if completed != 1 {
		t.Fatalf("exercises_completed = %d, want 1", completed)
	}
	mustContain(t, c.get("/exercises?topic=slices-arrays").Body, "Completed")
}

func TestExerciseValidationAndAIErrors(t *testing.T) {
	fake := &fakeAI{}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Val", "val@example.com")

	r := c.post("/exercises", url.Values{"topic_id": {"5"}, "difficulty": {"expert"}, "type": {"function"}})
	if r.Status != http.StatusUnprocessableEntity {
		t.Errorf("bad difficulty: %d", r.Status)
	}
	r = c.post("/exercises", url.Values{"topic_id": {"0"}, "difficulty": {"beginner"}, "type": {"function"}})
	if r.Status != http.StatusUnprocessableEntity {
		t.Errorf("missing topic: %d", r.Status)
	}

	fake.setFail(ai.ErrInvalidResponse)
	r = c.post("/exercises", url.Values{"topic_id": {"5"}, "difficulty": {"beginner"}, "type": {"function"}})
	if r.Status != http.StatusServiceUnavailable {
		t.Fatalf("AI failure: %d", r.Status)
	}
	mustContain(t, r.Body, "couldn&#39;t understand")
}

func TestExercisesArePrivate(t *testing.T) {
	env := newTestEnv(t, withAI(&fakeAI{}))
	owner := env.newClient(t)
	owner.register("Owner", "owner@example.com")
	path := generateExercise(t, owner)

	other := env.newClient(t)
	other.register("Other", "other@example.com")
	if r := other.get(path); r.Status != http.StatusNotFound {
		t.Errorf("view other's exercise: %d", r.Status)
	}
	if r := other.post(path+"/complete", nil); r.Status != http.StatusNotFound {
		t.Errorf("complete other's exercise: %d", r.Status)
	}
	if r := other.get("/exercises"); strings.Contains(r.Body, "Sum a slice") {
		t.Error("other user's exercise listed")
	}
}
