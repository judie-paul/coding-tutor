package exercises

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"codingtutor/internal/ai"
	"codingtutor/internal/users"
	"codingtutor/internal/view"
)

func codeErrorMessage() string {
	return fmt.Sprintf("Paste some code to review (up to %d characters).", MaxCodeLength)
}

// ReviewPage renders the free-form code review page.
func (h *Handlers) ReviewPage(w http.ResponseWriter, r *http.Request) {
	h.renderReviewPage(w, r, http.StatusOK, nil, nil, "")
}

func (h *Handlers) renderReviewPage(w http.ResponseWriter, r *http.Request, status int, form, errs map[string]string, aiErr string) {
	u := users.FromContext(r.Context())
	recent, err := h.Service.Submissions(r.Context(), u.ID, 0, 10)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.Render(w, r, status, "review/new", view.Page{
		Title: "Code review", Nav: "review", Form: form, Errors: errs,
		Data: map[string]any{"Recent": recent, "AIError": aiErr},
	})
}

// SubmitReview reviews free-form code.
func (h *Handlers) SubmitReview(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	code := r.PostFormValue("code")
	id, err := h.Service.Submit(r.Context(), u, 0, code)
	form := map[string]string{"code": code}
	switch {
	case errors.Is(err, ErrInvalidCode):
		h.renderReviewPage(w, r, http.StatusUnprocessableEntity, form, map[string]string{"code": codeErrorMessage()}, "")
		return
	case errors.Is(err, ErrInvalidInput):
		h.renderReviewPage(w, r, http.StatusUnprocessableEntity, form, map[string]string{"code": "Choose a programming language in settings first."}, "")
		return
	case ai.IsAIError(err):
		h.renderReviewPage(w, r, http.StatusServiceUnavailable, form, nil, ai.UserMessage(err))
		return
	case err != nil:
		h.View.ServerError(w, r, err)
		return
	}
	view.Redirect(w, r, fmt.Sprintf("/review/%d", id))
}

// SubmitSolution submits code for an exercise and reviews it.
func (h *Handlers) SubmitSolution(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	e, ok := h.load(w, r, u.ID)
	if !ok {
		return
	}
	code := r.PostFormValue("code")
	id, err := h.Service.Submit(r.Context(), u, e.ID, code)
	form := map[string]string{"code": code}
	switch {
	case errors.Is(err, ErrInvalidCode) || errors.Is(err, ErrInvalidInput):
		h.renderShow(w, r, http.StatusUnprocessableEntity, form, func(d *showData) { d.SubmitError = codeErrorMessage() })
		return
	case ai.IsAIError(err):
		h.renderShow(w, r, http.StatusServiceUnavailable, form, func(d *showData) { d.SubmitError = ai.UserMessage(err) })
		return
	case err != nil:
		h.View.ServerError(w, r, err)
		return
	}
	view.Redirect(w, r, fmt.Sprintf("/review/%d", id))
}

// ShowReview renders a submission and its review.
func (h *Handlers) ShowReview(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		h.View.NotFound(w, r)
		return
	}
	sub, err := h.Service.GetSubmission(r.Context(), u.ID, id)
	if errors.Is(err, ErrSubmissionNotFound) {
		h.View.NotFound(w, r)
		return
	}
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	var ex *Exercise
	if sub.ExerciseID != 0 {
		if ex, err = h.Service.Get(r.Context(), u.ID, sub.ExerciseID); err != nil {
			h.View.ServerError(w, r, err)
			return
		}
	}
	nav := "review"
	if ex != nil {
		nav = "exercises"
	}
	h.View.Render(w, r, http.StatusOK, "review/show", view.Page{
		Title: "Code review", Nav: nav,
		Data: map[string]any{"Submission": sub, "Exercise": ex},
	})
}
