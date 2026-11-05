package tutor

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"codingtutor/internal/ai"
	"codingtutor/internal/middleware"
	"codingtutor/internal/topics"
	"codingtutor/internal/users"
	"codingtutor/internal/view"
)

// Handlers serves the AI tutor pages.
type Handlers struct {
	Service *Service
	Topics  *topics.Store
	View    *view.Renderer
}

// Default conversation starters shown on the new-conversation page.
var defaultSuggestions = []string{
	"Explain goroutines like I'm a beginner.",
	"What is the difference between an array and a slice?",
	"How do interfaces work in Go? Give a real-world analogy.",
	"Explain this code line by line: for i, v := range nums { total += v }",
}

type newPageData struct {
	Topics      []topics.Topic
	Recent      []Conversation
	Suggestions []string
	AIError     string
}

// New renders the page for starting a conversation.
func (h *Handlers) New(w http.ResponseWriter, r *http.Request) {
	form := map[string]string{"question": r.URL.Query().Get("q")}
	u := users.FromContext(r.Context())
	if slug := r.URL.Query().Get("topic"); slug != "" {
		if t, err := h.Topics.GetBySlug(r.Context(), u.LanguageID, slug); err == nil {
			form["topic_id"] = strconv.FormatInt(t.ID, 10)
			if form["question"] == "" {
				form["question"] = "Teach me about " + t.Title + ". Start with the basics and include a short example."
			}
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
	recent, err := h.Service.List(r.Context(), u.ID, 5)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.Render(w, r, status, "tutor/new", view.Page{
		Title: "AI Tutor", Nav: "tutor", Form: form, Errors: errs,
		Data: newPageData{Topics: ts, Recent: recent, Suggestions: defaultSuggestions, AIError: aiErr},
	})
}

// Create starts a new conversation with the first question.
func (h *Handlers) Create(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	form := view.FormValues(r, "topic_id", "question")

	var topicID int64
	var topicTitle string
	if id, _ := strconv.ParseInt(form["topic_id"], 10, 64); id != 0 {
		t, err := h.Topics.Get(r.Context(), id)
		if err != nil || t.LanguageID != u.LanguageID {
			h.renderNew(w, r, http.StatusUnprocessableEntity, form, map[string]string{"topic_id": "Choose a topic from the list."}, "")
			return
		}
		topicID, topicTitle = t.ID, t.Title
	}

	convID, err := h.Service.Start(r.Context(), u, topicID, topicTitle, form["question"])
	switch {
	case errors.Is(err, ErrInvalidQuestion):
		h.renderNew(w, r, http.StatusUnprocessableEntity, form,
			map[string]string{"question": fmt.Sprintf("Please type a question (up to %d characters).", MaxQuestionLength)}, "")
		return
	case ai.IsAIError(err):
		h.renderNew(w, r, http.StatusServiceUnavailable, form, nil, ai.UserMessage(err))
		return
	case err != nil:
		h.View.ServerError(w, r, err)
		return
	}
	view.Redirect(w, r, fmt.Sprintf("/tutor/c/%d", convID))
}

type showPageData struct {
	Conversation Conversation
	Messages     []Message
	AIError      string
}

// Show renders a conversation.
func (h *Handlers) Show(w http.ResponseWriter, r *http.Request) {
	h.renderShow(w, r, http.StatusOK, nil, "")
}

func (h *Handlers) renderShow(w http.ResponseWriter, r *http.Request, status int, form map[string]string, aiErr string) {
	u := users.FromContext(r.Context())
	id, ok := pathID(r)
	if !ok {
		h.View.NotFound(w, r)
		return
	}
	conv, err := h.Service.Get(r.Context(), u.ID, id)
	if errors.Is(err, ErrNotFound) {
		h.View.NotFound(w, r)
		return
	}
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	msgs, err := h.Service.Messages(r.Context(), u.ID, id)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.Render(w, r, status, "tutor/conversation", view.Page{
		Title: conv.Title, Nav: "history", Form: form,
		Data: showPageData{Conversation: conv, Messages: msgs, AIError: aiErr},
	})
}

type messageJSON struct {
	Text string `json:"text,omitempty"`
	HTML string `json:"html,omitempty"`
	Time string `json:"time"`
}

// Ask posts a follow-up question. It answers JSON for fetch() requests and
// falls back to a redirect for plain form posts.
func (h *Handlers) Ask(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	id, ok := pathID(r)
	if !ok {
		h.View.NotFound(w, r)
		return
	}
	question := r.PostFormValue("question")
	userMsg, reply, err := h.Service.Ask(r.Context(), u, id, question)
	wantsJSON := middleware.WantsJSON(r)

	switch {
	case errors.Is(err, ErrNotFound):
		h.View.NotFound(w, r)
		return
	case errors.Is(err, ErrInvalidQuestion):
		msg := fmt.Sprintf("Please type a question (up to %d characters).", MaxQuestionLength)
		if wantsJSON {
			view.JSON(w, http.StatusUnprocessableEntity, map[string]string{"error": msg})
			return
		}
		h.renderShow(w, r, http.StatusUnprocessableEntity, map[string]string{"question": question}, msg)
		return
	case ai.IsAIError(err):
		if wantsJSON {
			view.JSON(w, http.StatusServiceUnavailable, map[string]string{"error": ai.UserMessage(err)})
			return
		}
		h.renderShow(w, r, http.StatusServiceUnavailable, map[string]string{"question": question}, ai.UserMessage(err))
		return
	case err != nil:
		h.View.ServerError(w, r, err)
		return
	}

	if wantsJSON {
		view.JSON(w, http.StatusOK, map[string]messageJSON{
			"user":      {Text: userMsg.Content, Time: view.TimeAgo(userMsg.CreatedAt)},
			"assistant": {HTML: string(view.Markdown(reply.Content)), Time: view.TimeAgo(reply.CreatedAt)},
		})
		return
	}
	view.Redirect(w, r, fmt.Sprintf("/tutor/c/%d#latest", id))
}

// topicGroup is a set of conversations about one topic.
type topicGroup struct {
	Title         string
	Slug          string
	Conversations []Conversation
}

// History lists all conversations grouped by topic.
func (h *Handlers) History(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	convs, err := h.Service.List(r.Context(), u.ID, 0)
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	var groups []*topicGroup
	index := map[string]*topicGroup{}
	for _, c := range convs {
		key := c.TopicSlug
		g, ok := index[key]
		if !ok {
			title := c.TopicTitle
			if title == "" {
				title = "General questions"
			}
			g = &topicGroup{Title: title, Slug: c.TopicSlug}
			index[key] = g
			groups = append(groups, g)
		}
		g.Conversations = append(g.Conversations, c)
	}
	h.View.Render(w, r, http.StatusOK, "tutor/history", view.Page{
		Title: "Conversation history", Nav: "history",
		Data: map[string]any{"Groups": groups, "Total": len(convs)},
	})
}

// Delete removes a conversation.
func (h *Handlers) Delete(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	id, ok := pathID(r)
	if !ok {
		h.View.NotFound(w, r)
		return
	}
	if err := h.Service.Delete(r.Context(), u.ID, id); errors.Is(err, ErrNotFound) {
		h.View.NotFound(w, r)
		return
	} else if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.SetFlash(w, "info", "Conversation deleted.")
	view.Redirect(w, r, "/tutor/history")
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}
