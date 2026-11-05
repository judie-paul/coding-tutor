package quizzes

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"codingtutor/internal/ai"
	"codingtutor/internal/topics"
	"codingtutor/internal/users"
	"codingtutor/internal/view"
)

// Handlers serves the quiz pages.
type Handlers struct {
	Service *Service
	Topics  *topics.Store
	View    *view.Renderer
}

// List shows the student's quizzes and recent results.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	list, err := h.Service.List(r.Context(), u.ID)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	recent, err := h.Service.RecentAttempts(r.Context(), u.ID, 5)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.Render(w, r, http.StatusOK, "quizzes/list", view.Page{
		Title: "Quizzes", Nav: "quizzes",
		Data: map[string]any{"Quizzes": list, "Recent": recent},
	})
}

type newPageData struct {
	Topics       []topics.Topic
	Difficulties []string
	Counts       []int
	AIError      string
}

// New renders the quiz generator.
func (h *Handlers) New(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	form := map[string]string{"difficulty": u.ExperienceLevel, "count": "5"}
	if slug := r.URL.Query().Get("topic"); slug != "" {
		if t, err := h.Topics.GetBySlug(r.Context(), u.LanguageID, slug); err == nil {
			form["topic_id"] = strconv.FormatInt(t.ID, 10)
		}
	}
	h.renderNew(w, r, http.StatusOK, form, nil, "")
}

func (h *Handlers) renderNew(w http.ResponseWriter, r *http.Request, status int, form, errs map[string]string, aiErr string) {
	u := users.FromContext(r.Context())
	ts, err := h.Topics.List(r.Context(), u.LanguageID)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.Render(w, r, status, "quizzes/new", view.Page{
		Title: "New quiz", Nav: "quizzes", Form: form, Errors: errs,
		Data: newPageData{Topics: ts, Difficulties: users.Levels, Counts: QuestionCounts, AIError: aiErr},
	})
}

// Create generates a quiz.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	form := view.FormValues(r, "topic_id", "difficulty", "count")
	topicID, _ := strconv.ParseInt(form["topic_id"], 10, 64)
	t, err := h.Topics.Get(r.Context(), topicID)
	if err != nil || t.LanguageID != u.LanguageID {
		h.renderNew(w, r, http.StatusUnprocessableEntity, form, map[string]string{"topic_id": "Choose a topic."}, "")
		return
	}
	count, _ := strconv.Atoi(form["count"])
	id, err := h.Service.Generate(r.Context(), u, t, form["difficulty"], count)
	switch {
	case errors.Is(err, ErrInvalidInput):
		h.renderNew(w, r, http.StatusUnprocessableEntity, form, map[string]string{"options": "Choose a difficulty and number of questions."}, "")
		return
	case ai.IsAIError(err):
		h.renderNew(w, r, http.StatusServiceUnavailable, form, nil, ai.UserMessage(err))
		return
	case err != nil:
		h.View.ServerError(w, r, err)
		return
	}
	view.Redirect(w, r, fmt.Sprintf("/quizzes/%d", id))
}

// TakeQuestion is what the quiz-taking page receives. It deliberately has
// no answer or explanation fields, so answers cannot leak into the page.
type TakeQuestion struct {
	ID       int64
	Position int
	Type     string
	Prompt   string
	Options  []string
}

// Take renders the quiz for answering.
func (h *Handlers) Take(w http.ResponseWriter, r *http.Request) {
	h.renderTake(w, r, http.StatusOK, nil)
}

func (h *Handlers) renderTake(w http.ResponseWriter, r *http.Request, status int, form map[string]string) {
	u := users.FromContext(r.Context())
	q, ok := h.load(w, r, u.ID)
	if !ok {
		return
	}
	qs := make([]TakeQuestion, 0, len(q.Questions))
	for _, qu := range q.Questions {
		qs = append(qs, TakeQuestion{ID: qu.ID, Position: qu.Position, Type: qu.Type, Prompt: qu.Prompt, Options: qu.Options})
	}
	h.View.Render(w, r, status, "quizzes/take", view.Page{
		Title: q.Title, Nav: "quizzes", Form: form,
		Data: map[string]any{
			"ID": q.ID, "Title": q.Title, "TopicTitle": q.TopicTitle, "TopicSlug": q.TopicSlug,
			"Difficulty": q.Difficulty, "Attempts": q.Attempts, "BestPercent": q.BestPercent, "Questions": qs,
		},
	})
}

func (h *Handlers) load(w http.ResponseWriter, r *http.Request, userID int64) (*Quiz, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.View.NotFound(w, r)
		return nil, false
	}
	q, err := h.Service.Get(r.Context(), userID, id)
	if errors.Is(err, ErrNotFound) {
		h.View.NotFound(w, r)
		return nil, false
	}
	if err != nil {
		h.View.ServerError(w, r, err)
		return nil, false
	}
	return q, true
}

// Submit grades the quiz and shows the results.
func (h *Handlers) Submit(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	q, ok := h.load(w, r, u.ID)
	if !ok {
		return
	}
	answers := make(map[int64]string, len(q.Questions))
	for _, qu := range q.Questions {
		answers[qu.ID] = r.PostFormValue(fmt.Sprintf("q_%d", qu.ID))
	}
	attemptID, err := h.Service.Submit(r.Context(), u, q.ID, answers)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	view.Redirect(w, r, fmt.Sprintf("/quizzes/attempts/%d", attemptID))
}

// Results shows a graded attempt with answers and explanations.
func (h *Handlers) Results(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.View.NotFound(w, r)
		return
	}
	a, err := h.Service.GetAttempt(r.Context(), u.ID, id)
	if errors.Is(err, ErrAttemptNotFound) {
		h.View.NotFound(w, r)
		return
	}
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.Render(w, r, http.StatusOK, "quizzes/results", view.Page{
		Title: "Quiz results", Nav: "quizzes", Data: a,
	})
}
