package app_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"codingtutor/internal/ai"
)

func TestProgressiveHints(t *testing.T) {
	fake := &fakeAI{}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Hana", "hana@example.com")
	path := generateExercise(t, c)

	page := c.get(path)
	mustContain(t, page.Body, "Get hint 1 of 3", "0 of 3 used")

	for level := 1; level <= 3; level++ {
		r := c.post(path+"/hints", url.Values{"current_code": {"func Sum() {}"}})
		if r.Status != http.StatusSeeOther {
			t.Fatalf("hint %d: %d\n%s", level, r.Status, r.Body)
		}
		// Each hint request must tell the AI which level it is and what was already shown.
		prompt := fake.lastUserPrompt()
		if !strings.Contains(prompt, "func Sum() {}") {
			t.Errorf("hint %d: student code not passed to AI", level)
		}
		if level > 1 && !strings.Contains(prompt, "Hints already given") {
			t.Errorf("hint %d: previous hints not passed to AI", level)
		}
	}
	page = c.get(path)
	mustContain(t, page.Body, "Hint 1", "Hint 3", "3 of 3 used", "Reveal the full solution")
	if strings.Contains(page.Body, "Get hint 4") {
		t.Error("no more than three hints should be offered")
	}

	// A fourth request does not create another hint.
	c.post(path+"/hints", nil)
	var hints, counted int
	env.deps.DB.QueryRow(`SELECT COUNT(*) FROM exercise_hints WHERE kind = 'hint'`).Scan(&hints)
	env.deps.DB.QueryRow(`SELECT hints_requested FROM learning_progress`).Scan(&counted)
	if hints != 3 || counted != 3 {
		t.Fatalf("hints stored=%d counted=%d, want 3/3", hints, counted)
	}
}

func TestSolutionRequiresExplicitConfirmation(t *testing.T) {
	fake := &fakeAI{}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Sol", "sol@example.com")
	path := generateExercise(t, c)

	r := c.post(path+"/solution", nil)
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("unconfirmed solution request: %d", r.Status)
	}
	if strings.Contains(r.Body, "Full solution") {
		t.Fatal("solution revealed without confirmation")
	}

	r = c.post(path+"/solution", url.Values{"confirm": {"yes"}})
	if r.Status != http.StatusSeeOther {
		t.Fatalf("confirmed solution: %d", r.Status)
	}
	page := c.get(path)
	mustContain(t, page.Body, "Full solution", "has not been compiled or run")

	calls := fake.callCount()
	c.post(path+"/solution", url.Values{"confirm": {"yes"}})
	if fake.callCount() != calls {
		t.Error("revealing again should reuse the stored solution")
	}
}

func TestHintFallsBackWhenAIUnavailable(t *testing.T) {
	fake := &fakeAI{}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Fay", "fay@example.com")
	path := generateExercise(t, c)

	fake.setFail(ai.ErrUnavailable)
	if r := c.post(path+"/hints", nil); r.Status != http.StatusSeeOther {
		t.Fatalf("fallback hint: %d", r.Status)
	}
	mustContain(t, c.get(path).Body, "Start with a total of zero.")

	// The solution has no fallback: the student sees a friendly error.
	r := c.post(path+"/solution", url.Values{"confirm": {"yes"}})
	if r.Status != http.StatusServiceUnavailable {
		t.Fatalf("solution with AI down: %d", r.Status)
	}
	mustContain(t, r.Body, "unavailable right now")
}

func TestHintsArePrivate(t *testing.T) {
	env := newTestEnv(t, withAI(&fakeAI{}))
	owner := env.newClient(t)
	owner.register("O", "o@example.com")
	path := generateExercise(t, owner)

	other := env.newClient(t)
	other.register("X", "x@example.com")
	if r := other.post(path+"/hints", nil); r.Status != http.StatusNotFound {
		t.Errorf("hint on other's exercise: %d", r.Status)
	}
	if r := other.post(path+"/solution", url.Values{"confirm": {"yes"}}); r.Status != http.StatusNotFound {
		t.Errorf("solution on other's exercise: %d", r.Status)
	}
}
