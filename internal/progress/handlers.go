package progress

import (
	"net/http"
	"strconv"

	"codingtutor/internal/ai"
	"codingtutor/internal/users"
	"codingtutor/internal/view"
)

const historyPageSize = 20

// Handlers serves the learning progress page.
type Handlers struct {
	Store *Store
	AI    ai.Tutor
	View  *view.Renderer
}

type pageData struct {
	Totals          Totals
	Topics          []TopicProgress
	RecentQuizzes   []QuizResult
	Struggles       []Struggle
	Revisit         []TopicProgress
	History         []Activity
	Page            int
	HasPrev         bool
	HasNext         bool
	Recommendations *ai.Recommendations
	AIError         string
}

// Show renders the progress page.
func (h *Handlers) Show(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, nil, "")
}

func (h *Handlers) load(r *http.Request) (pageData, []TopicProgress, error) {
	u := users.FromContext(r.Context())
	ctx := r.Context()
	var d pageData
	tps, err := h.Store.ByTopic(ctx, u.ID, u.LanguageID)
	if err != nil {
		return d, nil, err
	}
	d.Topics = tps
	d.Totals = Summarize(tps)
	d.Struggles = Struggles(tps)
	d.Revisit = Revisit(tps, 5)
	if d.RecentQuizzes, err = h.Store.RecentQuizzes(ctx, u.ID, 6); err != nil {
		return d, nil, err
	}

	d.Page, _ = strconv.Atoi(r.URL.Query().Get("page"))
	if d.Page < 1 {
		d.Page = 1
	}
	total, err := h.Store.CountActivity(ctx, u.ID)
	if err != nil {
		return d, nil, err
	}
	if d.History, err = h.Store.RecentActivity(ctx, u.ID, historyPageSize, (d.Page-1)*historyPageSize); err != nil {
		return d, nil, err
	}
	d.HasPrev = d.Page > 1
	d.HasNext = d.Page*historyPageSize < total
	return d, tps, nil
}

func (h *Handlers) render(w http.ResponseWriter, r *http.Request, status int, recs *ai.Recommendations, aiErr string) {
	d, _, err := h.load(r)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	d.Recommendations, d.AIError = recs, aiErr
	h.View.Render(w, r, status, "progress/progress", view.Page{Title: "Learning progress", Nav: "progress", Data: d})
}

// Recommend asks the AI study coach for advice based on recorded progress.
// The advice is shown on the page but never changes any statistics.
func (h *Handlers) Recommend(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	_, tps, err := h.load(r)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	titles := make([]string, 0, len(tps))
	for _, tp := range tps {
		titles = append(titles, tp.Title)
	}
	recs, err := h.AI.Recommend(r.Context(), ai.RecommendRequest{
		Learner: ai.Learner{Language: u.LanguageName, Level: u.ExperienceLevel},
		Topics:  titles,
		Summary: FactSheet(tps),
	})
	if err != nil {
		if !ai.IsAIError(err) {
			h.View.ServerError(w, r, err)
			return
		}
		h.render(w, r, http.StatusServiceUnavailable, nil, ai.UserMessage(err))
		return
	}
	h.render(w, r, http.StatusOK, recs, "")
}
