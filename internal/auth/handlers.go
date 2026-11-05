package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"codingtutor/internal/topics"
	"codingtutor/internal/users"
	"codingtutor/internal/view"
)

// Handlers serves the registration, login and logout pages.
type Handlers struct {
	Users    *users.Service
	Sessions *Sessions
	Topics   *topics.Store
	View     *view.Renderer
}

type registerData struct {
	Languages []topics.Language
	Levels    []string
}

// ShowRegister renders the sign-up form.
func (h *Handlers) ShowRegister(w http.ResponseWriter, r *http.Request) {
	if users.FromContext(r.Context()) != nil {
		view.Redirect(w, r, "/dashboard")
		return
	}
	h.renderRegister(w, r, http.StatusOK, map[string]string{"experience_level": users.LevelBeginner}, nil)
}

func (h *Handlers) renderRegister(w http.ResponseWriter, r *http.Request, status int, form, errs map[string]string) {
	langs, err := h.Topics.Languages(r.Context())
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.Render(w, r, status, "auth/register", view.Page{
		Title: "Create your account", Form: form, Errors: errs,
		Data: registerData{Languages: langs, Levels: users.Levels},
	})
}

// Register creates an account and logs the new student in.
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	form := view.FormValues(r, "name", "email", "language_id", "experience_level")
	langID, _ := strconv.ParseInt(form["language_id"], 10, 64)
	if langID == 0 {
		if id, err := h.Topics.DefaultLanguageID(r.Context()); err == nil {
			langID = id
		}
	}
	u, err := h.Users.Register(r.Context(), users.RegisterInput{
		Name: form["name"], Email: form["email"], Password: r.PostFormValue("password"),
		LanguageID: langID, ExperienceLevel: form["experience_level"],
	})
	var verrs users.ValidationErrors
	switch {
	case errors.As(err, &verrs):
		h.renderRegister(w, r, http.StatusUnprocessableEntity, form, verrs)
		return
	case errors.Is(err, users.ErrEmailTaken):
		h.renderRegister(w, r, http.StatusUnprocessableEntity, form,
			map[string]string{"email": "An account with this email already exists. Try logging in."})
		return
	case err != nil:
		h.View.ServerError(w, r, err)
		return
	}
	if err := h.Sessions.Create(r.Context(), w, r, u.ID); err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.SetFlash(w, "success", "Welcome, "+u.FirstName()+"! Your account is ready.")
	view.Redirect(w, r, "/dashboard")
}

// ShowLogin renders the login form.
func (h *Handlers) ShowLogin(w http.ResponseWriter, r *http.Request) {
	if users.FromContext(r.Context()) != nil {
		view.Redirect(w, r, "/dashboard")
		return
	}
	h.View.Render(w, r, http.StatusOK, "auth/login", view.Page{
		Title: "Log in", Form: map[string]string{"next": r.URL.Query().Get("next")},
	})
}

// Login verifies credentials and starts a session.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	form := view.FormValues(r, "email", "next")
	u, err := h.Users.Authenticate(r.Context(), form["email"], r.PostFormValue("password"))
	if errors.Is(err, users.ErrInvalidCredentials) {
		h.View.Render(w, r, http.StatusUnauthorized, "auth/login", view.Page{
			Title: "Log in", Form: form,
			Errors: map[string]string{"login": "That email and password combination is not correct."},
		})
		return
	}
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	if err := h.Sessions.Create(r.Context(), w, r, u.ID); err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	view.Redirect(w, r, SafeNext(form["next"]))
}

// Logout ends the session.
func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) {
	h.Sessions.Destroy(r.Context(), w, r)
	h.View.SetFlash(w, "info", "You have been logged out.")
	view.Redirect(w, r, "/login")
}

// SafeNext returns next if it is a local path, otherwise /dashboard.
// This prevents open redirects to other sites.
func SafeNext(next string) string {
	if next == "" || !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") ||
		strings.HasPrefix(next, "/\\") || strings.ContainsAny(next, "\r\n") {
		return "/dashboard"
	}
	return next
}
