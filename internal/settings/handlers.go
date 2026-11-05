// Package settings serves the account settings page.
package settings

import (
	"errors"
	"net/http"
	"strconv"

	"codingtutor/internal/auth"
	"codingtutor/internal/topics"
	"codingtutor/internal/users"
	"codingtutor/internal/view"
)

// Handlers serves account settings.
type Handlers struct {
	Users    *users.Service
	Sessions *auth.Sessions
	Topics   *topics.Store
	View     *view.Renderer
}

type pageData struct {
	Languages []topics.Language
	Levels    []string
}

func (h *Handlers) render(w http.ResponseWriter, r *http.Request, status int, form, errs map[string]string) {
	u := users.FromContext(r.Context())
	langs, err := h.Topics.Languages(r.Context())
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	if form == nil {
		form = map[string]string{
			"name":             u.Name,
			"language_id":      strconv.FormatInt(u.LanguageID, 10),
			"experience_level": u.ExperienceLevel,
		}
	}
	h.View.Render(w, r, status, "settings/settings", view.Page{
		Title: "Account settings", Nav: "settings", Form: form, Errors: errs,
		Data: pageData{Languages: langs, Levels: users.Levels},
	})
}

// Show renders the settings page.
func (h *Handlers) Show(w http.ResponseWriter, r *http.Request) {
	h.render(w, r, http.StatusOK, nil, nil)
}

// UpdateProfile saves name, language and experience level.
func (h *Handlers) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	form := view.FormValues(r, "name", "language_id", "experience_level")
	langID, _ := strconv.ParseInt(form["language_id"], 10, 64)
	err := h.Users.UpdateProfile(r.Context(), u.ID, users.ProfileInput{
		Name: form["name"], LanguageID: langID, ExperienceLevel: form["experience_level"],
	})
	var verrs users.ValidationErrors
	if errors.As(err, &verrs) {
		h.render(w, r, http.StatusUnprocessableEntity, form, verrs)
		return
	}
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.SetFlash(w, "success", "Your profile has been updated.")
	view.Redirect(w, r, "/settings")
}

// ChangePassword updates the password and signs out other devices.
func (h *Handlers) ChangePassword(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	err := h.Users.ChangePassword(r.Context(), u.ID, r.PostFormValue("current_password"),
		r.PostFormValue("new_password"), r.PostFormValue("confirm_password"))
	var verrs users.ValidationErrors
	if errors.As(err, &verrs) {
		h.render(w, r, http.StatusUnprocessableEntity, nil, verrs)
		return
	}
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	if err := h.Sessions.DestroyOthers(r.Context(), r, u.ID); err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.View.SetFlash(w, "success", "Password changed. Any other devices have been signed out.")
	view.Redirect(w, r, "/settings")
}

// DeleteAccount permanently removes the account and all learning data.
func (h *Handlers) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	u := users.FromContext(r.Context())
	err := h.Users.Delete(r.Context(), u.ID, r.PostFormValue("delete_password"))
	var verrs users.ValidationErrors
	if errors.As(err, &verrs) {
		h.render(w, r, http.StatusUnprocessableEntity, nil, verrs)
		return
	}
	if err != nil {
		h.View.ServerError(w, r, err)
		return
	}
	h.Sessions.Destroy(r.Context(), w, r)
	h.View.SetFlash(w, "info", "Your account and learning history have been deleted.")
	view.Redirect(w, r, "/login")
}
