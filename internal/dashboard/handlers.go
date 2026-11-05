// Package dashboard serves the student's home page.
package dashboard

import (
	"net/http"

	"codingtutor/internal/progress"
	"codingtutor/internal/users"
	"codingtutor/internal/view"
)

// Handlers serves the dashboard.
type Handlers struct {
	Progress *progress.Store
	View     *view.Renderer
}

type pageData struct {
	Totals      progress.Totals
	Recent      []progress.Activity
	Recommended []progress.TopicProgress
	HasActivity bool
}

// Show renders the dashboard from the student's recorded activity.
func (h *Handlers) Show(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	ctx := r.Context()

	var data pageData
	if u.LanguageID != 0 {
		tps, err := h.Progress.ByTopic(ctx, u.ID, u.LanguageID)
		if err != nil {
			h.View.ServerError(w, r, err)
			return
		}
		data.Totals = progress.Summarize(tps)
		data.Recommended = progress.Recommend(tps, 3)
	}
	recent, err := h.Progress.RecentActivity(ctx, u.ID, 8, 0)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	data.Recent = recent
	data.HasActivity = len(recent) > 0

	h.View.Render(w, r, http.StatusOK, "dashboard/dashboard", view.Page{
		Title: "Dashboard", Nav: "dashboard", Data: data,
	})
}
