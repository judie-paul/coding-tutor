package app_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"codingtutor/internal/ai"
	"codingtutor/internal/app"
)

func withAI(f *fakeAI) func(*app.Deps) {
	return func(d *app.Deps) { d.AI = f }
}

var convPath = regexp.MustCompile(`^/tutor/c/(\d+)$`)

func startConversation(t *testing.T, c *client, question string) string {
	t.Helper()
	r := c.post("/tutor", url.Values{"question": {question}, "topic_id": {"1"}})
	if r.Status != http.StatusSeeOther || !convPath.MatchString(r.Location) {
		t.Fatalf("start conversation: %d %q\n%s", r.Status, r.Location, r.Body)
	}
	return r.Location
}

func TestTutorConversationFlow(t *testing.T) {
	fake := &fakeAI{}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Alan", "alan@example.com")

	path := startConversation(t, c, "Explain goroutines like I'm a beginner.")
	page := c.get(path)
	if page.Status != http.StatusOK {
		t.Fatalf("conversation page: %d", page.Status)
	}
	mustContain(t, page.Body, "Explain goroutines like I&#39;m a beginner.", "Goroutines are lightweight threads", `<div class="code-block">`)
	if strings.Contains(page.Body, "<script>alert(1)</script>") {
		t.Fatal("AI output must be escaped")
	}

	// Follow-up via fetch (JSON).
	r := c.postJSON(path+"/messages", url.Values{"question": {"And channels?"}})
	if r.Status != http.StatusOK {
		t.Fatalf("follow-up: %d %s", r.Status, r.Body)
	}
	var body map[string]map[string]string
	if err := json.Unmarshal([]byte(r.Body), &body); err != nil {
		t.Fatal(err)
	}
	if body["user"]["text"] != "And channels?" || !strings.Contains(body["assistant"]["html"], "lightweight") {
		t.Errorf("unexpected JSON: %v", body)
	}
	if strings.Contains(body["assistant"]["html"], "<script>") {
		t.Fatal("JSON HTML must be escaped")
	}

	// The follow-up must include earlier turns as context.
	fake.mu.Lock()
	last := fake.calls[len(fake.calls)-1].Messages
	fake.mu.Unlock()
	if len(last) != 4 || last[1].Content != "Explain goroutines like I'm a beginner." {
		t.Errorf("follow-up missing history: %d messages", len(last))
	}

	// History groups the conversation under its topic, and progress is recorded.
	hist := c.get("/tutor/history")
	mustContain(t, hist.Body, "Go Basics", "Explain goroutines", "4 messages")
	dash := c.get("/dashboard")
	mustContain(t, dash.Body, "Started a conversation")
}

func TestTutorAIFailureKeepsQuestion(t *testing.T) {
	fake := &fakeAI{}
	env := newTestEnv(t, withAI(fake))
	c := env.newClient(t)
	c.register("Barbara", "barbara@example.com")

	fake.setFail(ai.ErrTimeout)
	r := c.post("/tutor", url.Values{"question": {"What is a map?"}})
	if r.Status != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", r.Status)
	}
	mustContain(t, r.Body, "took too long", "What is a map?")
	var n int
	env.deps.DB.QueryRow(`SELECT COUNT(*) FROM conversations`).Scan(&n)
	if n != 0 {
		t.Fatal("no conversation should be stored when the AI fails")
	}

	fake.setFail(nil)
	path := startConversation(t, c, "What is a map?")
	fake.setFail(ai.ErrUnavailable)
	j := c.postJSON(path+"/messages", url.Values{"question": {"More?"}})
	if j.Status != http.StatusServiceUnavailable || !strings.Contains(j.Body, "unavailable") {
		t.Fatalf("expected JSON 503 error, got %d %s", j.Status, j.Body)
	}
}

func TestTutorValidation(t *testing.T) {
	env := newTestEnv(t, withAI(&fakeAI{}))
	c := env.newClient(t)
	c.register("Carl", "carl@example.com")
	r := c.post("/tutor", url.Values{"question": {"   "}})
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("empty question: expected 422, got %d", r.Status)
	}
	r = c.post("/tutor", url.Values{"question": {"hi"}, "topic_id": {"9999"}})
	if r.Status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown topic: expected 422, got %d", r.Status)
	}
}

func TestUsersCannotAccessOthersConversations(t *testing.T) {
	env := newTestEnv(t, withAI(&fakeAI{}))
	alice := env.newClient(t)
	alice.register("Alice", "alice@example.com")
	path := startConversation(t, alice, "Private question about pointers")

	mallory := env.newClient(t)
	mallory.register("Mallory", "mallory@example.com")
	if r := mallory.get(path); r.Status != http.StatusNotFound {
		t.Errorf("GET other user's conversation: expected 404, got %d", r.Status)
	}
	if r := mallory.post(path+"/messages", url.Values{"question": {"hi"}}); r.Status != http.StatusNotFound {
		t.Errorf("POST to other user's conversation: expected 404, got %d", r.Status)
	}
	if r := mallory.post(path+"/delete", nil); r.Status != http.StatusNotFound {
		t.Errorf("delete other user's conversation: expected 404, got %d", r.Status)
	}
	if r := mallory.get("/tutor/history"); strings.Contains(r.Body, "Private question") {
		t.Error("other user's conversation listed in history")
	}
	if r := alice.get(path); r.Status != http.StatusOK {
		t.Error("owner lost access")
	}
}

func TestAIRateLimit(t *testing.T) {
	env := newTestEnv(t, withAI(&fakeAI{}), func(d *app.Deps) { d.Config.AIRateLimit = 2 })
	c := env.newClient(t)
	c.register("Rita", "rita@example.com")
	for i := 0; i < 2; i++ {
		startConversation(t, c, "question")
	}
	r := c.post("/tutor", url.Values{"question": {"one too many"}})
	if r.Status != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", r.Status)
	}
}
