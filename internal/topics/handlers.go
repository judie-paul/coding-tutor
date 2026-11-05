package topics

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"codingtutor/internal/database"
	"codingtutor/internal/progress"
	"codingtutor/internal/users"
	"codingtutor/internal/view"
)

// Handlers serves the topic list and topic detail pages.
type Handlers struct {
	Store    *Store
	Progress *progress.Store
	View     *view.Renderer
}

type topicCard struct {
	Topic
	Progress progress.TopicProgress
}

// List shows every topic in the student's language with their progress.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	ts, err := h.Store.List(r.Context(), u.LanguageID)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	tps, err := h.Progress.ByTopic(r.Context(), u.ID, u.LanguageID)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	byID := map[int64]progress.TopicProgress{}
	for _, tp := range tps {
		byID[tp.TopicID] = tp
	}
	cards := make([]topicCard, 0, len(ts))
	for _, t := range ts {
		cards = append(cards, topicCard{Topic: t, Progress: byID[t.ID]})
	}
	h.View.Render(w, r, http.StatusOK, "topics/list", view.Page{
		Title: "Programming topics", Nav: "topics",
		Data: map[string]any{"Cards": cards, "Totals": progress.Summarize(tps)},
	})
}

// Show renders one topic with the student's progress and next steps.
func (h *Handlers) Show(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	t, err := h.Store.GetBySlug(r.Context(), u.LanguageID, r.PathValue("slug"))
	if errors.Is(err, ErrNotFound) {
		h.View.NotFound(w, r)
		return
	}
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	tp, err := h.Progress.ForTopic(r.Context(), u.ID, u.LanguageID, t.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		h.View.ServerError(w, r, err)
		return
	}
	all, err := h.Store.List(r.Context(), u.LanguageID)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	var prev, next *Topic
	for i := range all {
		if all[i].ID == t.ID {
			if i > 0 {
				prev = &all[i-1]
			}
			if i+1 < len(all) {
				next = &all[i+1]
			}
		}
	}
	h.View.Render(w, r, http.StatusOK, "topics/show", view.Page{
		Title: t.Title, Nav: "topics",
		Data: map[string]any{"Topic": t, "Progress": tp, "Prev": prev, "Next": next},
	})
}

// ToggleLesson marks the topic's lesson as completed (or not).
func (h *Handlers) ToggleLesson(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	t, err := h.Store.GetBySlug(r.Context(), u.LanguageID, r.PathValue("slug"))
	if errors.Is(err, ErrNotFound) {
		h.View.NotFound(w, r)
		return
	}
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	done := r.PostFormValue("completed") == "1"
	if err := h.setLesson(r.Context(), u.ID, t, done); err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	if done {
		h.View.SetFlash(w, "success", "Nice work! “"+t.Title+"” is marked as completed.")
	} else {
		h.View.SetFlash(w, "info", "“"+t.Title+"” is no longer marked as completed.")
	}
	view.Redirect(w, r, "/topics/"+t.Slug)
}

func (h *Handlers) setLesson(ctx context.Context, userID int64, t Topic, done bool) error {
	return database.InTx(ctx, h.Progress.DB(), func(tx *sql.Tx) error {
		if err := h.Progress.SetLessonCompleted(ctx, tx, userID, t.ID, done); err != nil {
			return err
		}
		if !done {
			return nil
		}
		return h.Progress.Log(ctx, tx, userID, t.ID, progress.KindLessonCompleted,
			"Completed the lesson “"+t.Title+"”", "/topics/"+t.Slug)
	})
}
