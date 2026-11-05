// Package app wires stores, services and handlers into an http.Handler.
package app

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"codingtutor/internal/auth"
	"codingtutor/internal/config"
	"codingtutor/internal/dashboard"
	"codingtutor/internal/middleware"
	"codingtutor/internal/progress"
	"codingtutor/internal/settings"
	"codingtutor/internal/topics"
	"codingtutor/internal/users"
	"codingtutor/internal/view"
	"codingtutor/web"
)

// Deps are the external dependencies of the application.
type Deps struct {
	Config config.Config
	DB     *sql.DB
	Logger *slog.Logger
	// BcryptCost overrides the password hashing cost (tests use the minimum).
	BcryptCost int
}

// App is the assembled application.
type App struct {
	Handler  http.Handler
	Sessions *auth.Sessions
}

// New builds the application's HTTP handler.
func New(d Deps) (*App, error) {
	cfg, logger := d.Config, d.Logger

	renderer, err := view.NewRenderer(web.FS, logger, cfg.SecureCookies)
	if err != nil {
		return nil, err
	}

	userStore := users.NewStore(d.DB)
	userService := users.NewService(userStore, d.BcryptCost)
	topicStore := topics.NewStore(d.DB)
	sessions := auth.NewSessions(d.DB, userStore, cfg.SessionTTL, cfg.SecureCookies, logger)

	authH := &auth.Handlers{Users: userService, Sessions: sessions, Topics: topicStore, View: renderer}
	progressStore := progress.NewStore(d.DB)
	dashboardH := &dashboard.Handlers{Progress: progressStore, View: renderer}
	settingsH := &settings.Handlers{Users: userService, Sessions: sessions, Topics: topicStore, View: renderer}

	// Limit login/registration attempts per client IP to slow down password guessing.
	authLimiter := middleware.NewRateLimiter(10, time.Minute)
	limitAuth := middleware.Limit(authLimiter,
		func(r *http.Request) string { return middleware.ClientIP(r, cfg.TrustProxy) },
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			renderer.Error(w, r, http.StatusTooManyRequests, "Too many attempts. Please wait a minute and try again.")
		}))

	mux := http.NewServeMux()
	protected := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuth(h) }

	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		return nil, err
	}
	mux.Handle("GET /static/", cacheStatic(http.StripPrefix("/static/", http.FileServerFS(staticFS))))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := d.DB.PingContext(r.Context()); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		if users.FromContext(r.Context()) != nil {
			view.Redirect(w, r, "/dashboard")
			return
		}
		renderer.Render(w, r, http.StatusOK, "auth/landing", view.Page{Title: "Learn to program"})
	})
	mux.HandleFunc("GET /register", authH.ShowRegister)
	mux.Handle("POST /register", limitAuth(http.HandlerFunc(authH.Register)))
	mux.HandleFunc("GET /login", authH.ShowLogin)
	mux.Handle("POST /login", limitAuth(http.HandlerFunc(authH.Login)))
	mux.HandleFunc("POST /logout", authH.Logout)

	mux.Handle("GET /dashboard", protected(dashboardH.Show))

	mux.Handle("GET /settings", protected(settingsH.Show))
	mux.Handle("POST /settings/profile", protected(settingsH.UpdateProfile))
	mux.Handle("POST /settings/password", protected(settingsH.ChangePassword))
	mux.Handle("POST /settings/delete", protected(settingsH.DeleteAccount))

	// Unknown routes get the friendly 404 page.
	mux.HandleFunc("/", renderer.NotFound)

	csrfFail := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderer.Error(w, r, http.StatusForbidden, "Your form expired or came from another site. Please go back, refresh the page and try again.")
	})

	handler := middleware.Chain(mux,
		middleware.Recover(logger),
		middleware.SecureHeaders,
		middleware.LimitBody(1<<20), // 1 MB is plenty for forms and code submissions
		sessions.LoadUser,
		middleware.Logging(logger),
		middleware.CSRF(cfg.SecureCookies, csrfFail),
	)
	return &App{Handler: handler, Sessions: sessions}, nil
}

func cacheStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		next.ServeHTTP(w, r)
	})
}
