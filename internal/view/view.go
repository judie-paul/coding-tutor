// Package view renders HTML templates and provides small helpers shared by
// HTTP handlers (flash messages, error pages, JSON responses).
package view

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"codingtutor/internal/middleware"
	"codingtutor/internal/users"
)

// Page is the data passed to every template.
type Page struct {
	Title  string
	Nav    string            // active sidebar item
	Data   any               // page-specific data
	Form   map[string]string // previously submitted values to re-display
	Errors map[string]string // field errors

	// Filled in by Render.
	User      *users.User
	CSRFToken string
	Flash     *Flash
	Year      int
}

// Flash is a one-time message shown on the next page view.
type Flash struct {
	Kind    string // success, error, info
	Message string
}

// Renderer holds parsed page templates.
type Renderer struct {
	pages  map[string]*template.Template
	logger *slog.Logger
	secure bool
}

// NewRenderer parses every page under templates/ in fsys. Each page is parsed
// together with the shared layouts and partials.
func NewRenderer(fsys fs.FS, logger *slog.Logger, secureCookies bool) (*Renderer, error) {
	r := &Renderer{pages: map[string]*template.Template{}, logger: logger, secure: secureCookies}
	shared := []string{"templates/layouts/*.html", "templates/partials/*.html"}

	err := fs.WalkDir(fsys, "templates", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || path.Ext(p) != ".html" {
			return nil
		}
		dir := path.Dir(p)
		if dir == "templates/layouts" || dir == "templates/partials" {
			return nil
		}
		name := strings.TrimSuffix(strings.TrimPrefix(p, "templates/"), ".html")
		t, err := template.New(name).Funcs(Funcs()).ParseFS(fsys, append(shared, p)...)
		if err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		r.pages[name] = t
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r, nil
}

// Render executes page into a buffer, then writes it with status. Writing via
// a buffer means a template error never sends a half-rendered page.
func (rd *Renderer) Render(w http.ResponseWriter, r *http.Request, status int, page string, p Page) {
	t, ok := rd.pages[page]
	if !ok {
		rd.logger.Error("unknown template", "page", page)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	p.User = users.FromContext(r.Context())
	p.CSRFToken = middleware.CSRFToken(r.Context())
	p.Flash = rd.popFlash(w, r)
	p.Year = time.Now().Year()
	if p.Form == nil {
		p.Form = map[string]string{}
	}
	if p.Errors == nil {
		p.Errors = map[string]string{}
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "base", p); err != nil {
		rd.logger.Error("render template", "page", page, "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// Error renders a friendly error page. message must be safe to show users.
func (rd *Renderer) Error(w http.ResponseWriter, r *http.Request, status int, message string) {
	if middleware.WantsJSON(r) {
		JSON(w, status, map[string]string{"error": message})
		return
	}
	rd.Render(w, r, status, "errors/error", Page{
		Title: http.StatusText(status),
		Data:  map[string]any{"Status": status, "Message": message},
	})
}

// NotFound renders the 404 page.
func (rd *Renderer) NotFound(w http.ResponseWriter, r *http.Request) {
	rd.Error(w, r, http.StatusNotFound, "We couldn't find that page. It may have been removed, or it may belong to another account.")
}

// ServerError logs err and renders a generic 500 page without internal details.
func (rd *Renderer) ServerError(w http.ResponseWriter, r *http.Request, err error) {
	rd.logger.Error("server error", "method", r.Method, "path", r.URL.Path, "err", err)
	rd.Error(w, r, http.StatusInternalServerError, "Something went wrong on our side. Please try again in a moment.")
}

// SetFlash stores a one-time message to show on the next rendered page.
func (rd *Renderer) SetFlash(w http.ResponseWriter, kind, message string) {
	v := base64.RawURLEncoding.EncodeToString([]byte(kind + "|" + message))
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: v, Path: "/", MaxAge: 60,
		HttpOnly: true, Secure: rd.secure, SameSite: http.SameSiteLaxMode})
}

func (rd *Renderer) popFlash(w http.ResponseWriter, r *http.Request) *Flash {
	c, err := r.Cookie("flash")
	if err != nil {
		return nil
	}
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: "", Path: "/", MaxAge: -1,
		HttpOnly: true, Secure: rd.secure, SameSite: http.SameSiteLaxMode})
	raw, err := base64.RawURLEncoding.DecodeString(c.Value)
	if err != nil {
		return nil
	}
	kind, msg, ok := strings.Cut(string(raw), "|")
	if !ok || msg == "" {
		return nil
	}
	switch kind {
	case "success", "error", "info":
	default:
		kind = "info"
	}
	return &Flash{Kind: kind, Message: msg}
}

// Redirect sends a 303 See Other redirect, the right status after a POST.
func Redirect(w http.ResponseWriter, r *http.Request, url string) {
	http.Redirect(w, r, url, http.StatusSeeOther)
}

// JSON writes v as a JSON response.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// FormValues copies the named form fields so a form can be re-displayed.
func FormValues(r *http.Request, names ...string) map[string]string {
	m := make(map[string]string, len(names))
	for _, n := range names {
		m[n] = r.PostFormValue(n)
	}
	return m
}
