package exercises

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

// Handlers serves the exercise pages.
type Handlers struct {
	Service *Service
	Topics  *topics.Store
	View    *view.Renderer
}

type option struct{ Value, Label string }

type newPageData struct {
	Topics       []topics.Topic
	Difficulties []string
	Types        []option
	AIError      string
}

func exerciseTypes() []option {
	out := make([]option, 0, len(ai.ExerciseTypes))
	for _, t := range ai.ExerciseTypes {
		out = append(out, option{t, ai.ExerciseTypeLabel(t)})
	}
	return out
}

// New renders the exercise generator.
func (h *Handlers) New(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	form := map[string]string{"difficulty": u.ExperienceLevel, "type": ai.ExerciseTypes[0]}
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
	h.View.Render(w, r, status, "exercises/new", view.Page{
		Title: "New exercise", Nav: "exercises", Form: form, Errors: errs,
		Data: newPageData{Topics: ts, Difficulties: users.Levels, Types: exerciseTypes(), AIError: aiErr},
	})
}

// Create generates and stores a new exercise.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	form := view.FormValues(r, "topic_id", "difficulty", "type")
	topicID, _ := strconv.ParseInt(form["topic_id"], 10, 64)
	t, err := h.Topics.Get(r.Context(), topicID)
	if err != nil || t.LanguageID != u.LanguageID {
		h.renderNew(w, r, http.StatusUnprocessableEntity, form, map[string]string{"topic_id": "Choose a topic."}, "")
		return
	}
	id, err := h.Service.Generate(r.Context(), u, t, form["difficulty"], form["type"])
	switch {
	case errors.Is(err, ErrInvalidInput):
		h.renderNew(w, r, http.StatusUnprocessableEntity, form, map[string]string{"options": "Choose a difficulty and an exercise type."}, "")
		return
	case ai.IsAIError(err):
		h.renderNew(w, r, http.StatusServiceUnavailable, form, nil, ai.UserMessage(err))
		return
	case err != nil:
		h.View.ServerError(w, r, err)
		return
	}
	view.Redirect(w, r, fmt.Sprintf("/exercises/%d", id))
}

// List shows the student's exercises, optionally filtered by topic.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	var filter *topics.Topic
	if slug := r.URL.Query().Get("topic"); slug != "" {
		t, err := h.Topics.GetBySlug(r.Context(), u.LanguageID, slug)
		if err == nil {
			filter = &t
		}
	}
	var topicID int64
	if filter != nil {
		topicID = filter.ID
	}
	list, err := h.Service.List(r.Context(), u.ID, topicID)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	completed := 0
	for _, e := range list {
		if e.Completed() {
			completed++
		}
	}
	h.View.Render(w, r, http.StatusOK, "exercises/list", view.Page{
		Title: "Exercises", Nav: "exercises",
		Data: map[string]any{"Exercises": list, "Filter": filter, "Completed": completed},
	})
}

// showData is the data for the exercise detail page.
type showData struct {
	Exercise  *Exercise
	Hints     []Hint
	HintsLeft int
	Solution  *Hint
	HintError string
}

// Show renders an exercise.
func (h *Handlers) Show(w http.ResponseWriter, r *http.Request) {
	h.renderShow(w, r, http.StatusOK, nil, nil)
}

func (h *Handlers) renderShow(w http.ResponseWriter, r *http.Request, status int, form map[string]string, extra func(*showData)) {
	u := users.FromContext(r.Context())
	e, ok := h.load(w, r, u.ID)
	if !ok {
		return
	}
	hints, err := h.Service.Hints(r.Context(), u.ID, e.ID)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	data := showData{Exercise: e, HintsLeft: MaxHints}
	for i := range hints {
		if hints[i].IsSolution() {
			data.Solution = &hints[i]
			continue
		}
		data.Hints = append(data.Hints, hints[i])
		data.HintsLeft--
	}
	if extra != nil {
		extra(&data)
	}
	h.View.Render(w, r, status, "exercises/show", view.Page{
		Title: e.Title, Nav: "exercises", Form: form, Data: data,
	})
}

// load fetches the exercise named in the URL, writing a 404 if needed.
func (h *Handlers) load(w http.ResponseWriter, r *http.Request, userID int64) (*Exercise, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.View.NotFound(w, r)
		return nil, false
	}
	e, err := h.Service.Get(r.Context(), userID, id)
	if errors.Is(err, ErrNotFound) {
		h.View.NotFound(w, r)
		return nil, false
	}
	if err != nil {
		h.View.ServerError(w, r, err)
		return nil, false
	}
	return e, true
}

// Complete marks the exercise as completed or understood.
func (h *Handlers) Complete(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	e, ok := h.load(w, r, u.ID)
	if !ok {
		return
	}
	if err := h.Service.MarkCompleted(r.Context(), u, e.ID); err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.SetFlash(w, "success", "Exercise marked as completed. Well done!")
	view.Redirect(w, r, fmt.Sprintf("/exercises/%d", e.ID))
}

// Hint reveals the next progressive hint.
func (h *Handlers) Hint(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	e, ok := h.load(w, r, u.ID)
	if !ok {
		return
	}
	code := r.PostFormValue("current_code")
	_, err := h.Service.RequestHint(r.Context(), u, e.ID, code)
	switch {
	case errors.Is(err, ErrNoMoreHints):
		h.View.SetFlash(w, "info", "You've used all three hints. You can still ask the tutor or view the full solution.")
	case ai.IsAIError(err):
		h.renderShow(w, r, http.StatusServiceUnavailable, map[string]string{"current_code": code},
			func(d *showData) { d.HintError = ai.UserMessage(err) })
		return
	case err != nil:
		h.View.ServerError(w, r, err)
		return
	}
	view.Redirect(w, r, fmt.Sprintf("/exercises/%d#hints", e.ID))
}

// Solution reveals the full solution, only when explicitly confirmed.
func (h *Handlers) Solution(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	e, ok := h.load(w, r, u.ID)
	if !ok {
		return
	}
	_, err := h.Service.RevealSolution(r.Context(), u, e.ID, r.PostFormValue("confirm") == "yes")
	switch {
	case errors.Is(err, ErrSolutionNotConfirmed):
		h.renderShow(w, r, http.StatusUnprocessableEntity, nil,
			func(d *showData) { d.HintError = "Tick the box to confirm you want to see the full solution." })
		return
	case ai.IsAIError(err):
		h.renderShow(w, r, http.StatusServiceUnavailable, nil, func(d *showData) { d.HintError = ai.UserMessage(err) })
		return
	case err != nil:
		h.View.ServerError(w, r, err)
		return
	}
	view.Redirect(w, r, fmt.Sprintf("/exercises/%d#solution", e.ID))
}
