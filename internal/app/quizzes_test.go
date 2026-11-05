package app_test

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

var quizPath = regexp.MustCompile(`^/quizzes/(\d+)$`)

func createQuiz(t *testing.T, c *client) string {
	t.Helper()
	r := c.post("/quizzes", url.Values{"topic_id": {"5"}, "difficulty": {"beginner"}, "count": {"5"}})
	if r.Status != http.StatusSeeOther || !quizPath.MatchString(r.Location) {
		t.Fatalf("create quiz: %d %q\n%s", r.Status, r.Location, r.Body)
	}
	return r.Location
}

// questionIDs returns the quiz's question IDs in order, keyed by question type.
func questionIDs(t *testing.T, env *testEnv) map[string]int64 {
	t.Helper()
	rows, err := env.deps.DB.Query(`SELECT id, question_type FROM quiz_questions ORDER BY position`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	ids := map[string]int64{}
	for rows.Next() {
		var id int64
		var typ string
		rows.Scan(&id, &typ)
		ids[typ] = id
	}
	return ids
}

func TestQuizTakingDoesNotRevealAnswers(t *testing.T) {
	env := newTestEnv(t, withAI(&fakeAI{}))
	c := env.newClient(t)
	c.register("Quinn", "quinn@example.com")
	path := createQuiz(t, c)

	page := c.get(path)
	if page.Status != http.StatusOK {
		t.Fatalf("take quiz: %d", page.Status)
	}
	mustContain(t, page.Body, "Slices check", "Which builtin adds to a slice?", "push", "Submit answers")
	for _, secret := range []string{"append returns a new slice", "Length is part of the type", "len works on slices", "Go has no push", "len()"} {
		if strings.Contains(page.Body, secret) {
			t.Errorf("quiz page leaks answer material: %q", secret)
		}
	}
}

func TestQuizGradingAndResults(t *testing.T) {
	env := newTestEnv(t, withAI(&fakeAI{}))
	c := env.newClient(t)
	c.register("Gina", "gina@example.com")
	path := createQuiz(t, c)
	ids := questionIDs(t, env)

	r := c.post(path+"/attempts", url.Values{
		fmt.Sprintf("q_%d", ids["multiple_choice"]): {"push"},  // wrong
		fmt.Sprintf("q_%d", ids["true_false"]):      {"True"},  // right
		fmt.Sprintf("q_%d", ids["short_answer"]):    {"LEN()"}, // accepted alternative
	})
	if r.Status != http.StatusSeeOther || !strings.HasPrefix(r.Location, "/quizzes/attempts/") {
		t.Fatalf("submit quiz: %d %q", r.Status, r.Location)
	}
	res := c.get(r.Location)
	mustContain(t, res.Body, "67%", "2 of 3 correct", "Go has no push.", "append returns a new slice.", "(your answer)")

	var best, last, taken int
	env.deps.DB.QueryRow(`SELECT best_quiz_percent, last_quiz_percent, quizzes_taken FROM learning_progress`).Scan(&best, &last, &taken)
	if best != 67 || last != 67 || taken != 1 {
		t.Fatalf("progress best=%d last=%d taken=%d", best, last, taken)
	}

	// A retake with everything right updates best and last scores.
	c.post(path+"/attempts", url.Values{
		fmt.Sprintf("q_%d", ids["multiple_choice"]): {"append"},
		fmt.Sprintf("q_%d", ids["true_false"]):      {"True"},
		fmt.Sprintf("q_%d", ids["short_answer"]):    {"len"},
	})
	env.deps.DB.QueryRow(`SELECT best_quiz_percent, quizzes_taken FROM learning_progress`).Scan(&best, &taken)
	if best != 100 || taken != 2 {
		t.Fatalf("after retake best=%d taken=%d", best, taken)
	}
	mustContain(t, c.get("/quizzes").Body, "Slices check", "100%")
	mustContain(t, c.get("/dashboard").Body, "Scored 100%")
}

func TestQuizValidationAndPrivacy(t *testing.T) {
	env := newTestEnv(t, withAI(&fakeAI{}))
	c := env.newClient(t)
	c.register("Ola", "ola@example.com")
	if r := c.post("/quizzes", url.Values{"topic_id": {"5"}, "difficulty": {"beginner"}, "count": {"99"}}); r.Status != http.StatusUnprocessableEntity {
		t.Errorf("bad count: %d", r.Status)
	}
	path := createQuiz(t, c)
	r := c.post(path+"/attempts", url.Values{})
	attempt := r.Location

	other := env.newClient(t)
	other.register("Pry", "pry@example.com")
	if res := other.get(path); res.Status != http.StatusNotFound {
		t.Errorf("view other's quiz: %d", res.Status)
	}
	if res := other.post(path+"/attempts", url.Values{}); res.Status != http.StatusNotFound {
		t.Errorf("submit other's quiz: %d", res.Status)
	}
	if res := other.get(attempt); res.Status != http.StatusNotFound {
		t.Errorf("view other's results: %d", res.Status)
	}
}
