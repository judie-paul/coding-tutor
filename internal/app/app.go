// Package app wires stores, services and handlers into an http.Handler.
package app

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"codingtutor/internal/ai"
	"codingtutor/internal/auth"
	"codingtutor/internal/config"
	"codingtutor/internal/dashboard"
	"codingtutor/internal/exercises"
	"codingtutor/internal/middleware"
	"codingtutor/internal/progress"
	"codingtutor/internal/quizzes"
	"codingtutor/internal/settings"
	"codingtutor/internal/topics"
	"codingtutor/internal/tutor"
	"codingtutor/internal/users"
	"codingtutor/internal/view"
	"codingtutor/web"
)

// Deps are the external dependencies of the application.
type Deps struct {
	Config config.Config
	DB     *sql.DB
	Logger *slog.Logger
	// AI overrides the model client (tests pass a fake). When nil, an
	// OpenAI-compatible client is built from Config.
	AI ai.Completer
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

	progressStore := progress.NewStore(d.DB)

	completer := d.AI
	if completer == nil {
		completer = ai.NewOpenAIClient(ai.ClientConfig{
			BaseURL: cfg.AIBaseURL, APIKey: cfg.AIAPIKey, Model: cfg.AIModel,
			Timeout: cfg.AITimeout, JSONMode: cfg.AIJSONMode,
		}, logger)
	}
	aiService := ai.NewService(completer, logger)
	tutorService := tutor.NewService(d.DB, aiService, progressStore)
	exerciseService := exercises.NewService(d.DB, aiService, progressStore)
	quizService := quizzes.NewService(d.DB, aiService, progressStore)

	authH := &auth.Handlers{Users: userService, Sessions: sessions, Topics: topicStore, View: renderer}
	dashboardH := &dashboard.Handlers{Progress: progressStore, View: renderer}
	settingsH := &settings.Handlers{Users: userService, Sessions: sessions, Topics: topicStore, View: renderer}
	tutorH := &tutor.Handlers{Service: tutorService, Topics: topicStore, View: renderer}
	topicsH := &topics.Handlers{Store: topicStore, Progress: progressStore, View: renderer}
	exercisesH := &exercises.Handlers{Service: exerciseService, Topics: topicStore, View: renderer}
	quizzesH := &quizzes.Handlers{Service: quizService, Topics: topicStore, View: renderer}
	progressH := &progress.Handlers{Store: progressStore, AI: aiService, View: renderer}

	// Limit login/registration attempts per client IP to slow down password guessing.
	authLimiter := middleware.NewRateLimiter(10, time.Minute)
	limitAuth := middleware.Limit(authLimiter,
		func(r *http.Request) string { return middleware.ClientIP(r, cfg.TrustProxy) },
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			renderer.Error(w, r, http.StatusTooManyRequests, "Too many attempts. Please wait a minute and try again.")
		}))

	// Limit AI requests per student: they cost money and take time.
	aiLimiter := middleware.NewRateLimiter(cfg.AIRateLimit, time.Minute)
	limitAI := middleware.Limit(aiLimiter,
		func(r *http.Request) string {
			return "user:" + strconv.FormatInt(users.FromContext(r.Context()).ID, 10)
		},
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			renderer.Error(w, r, http.StatusTooManyRequests, "You're sending requests to the AI tutor very quickly. Please wait a minute and try again.")
		}))

	mux := http.NewServeMux()
	protected := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuth(h) }
	// aiRoute requires login and applies the per-user AI rate limit.
	aiRoute := func(h http.HandlerFunc) http.Handler { return middleware.RequireAuth(limitAI(h)) }

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

	mux.Handle("GET /tutor", protected(tutorH.New))
	mux.Handle("POST /tutor", aiRoute(tutorH.Create))
	mux.Handle("GET /tutor/history", protected(tutorH.History))
	mux.Handle("GET /tutor/c/{id}", protected(tutorH.Show))
	mux.Handle("POST /tutor/c/{id}/messages", aiRoute(tutorH.Ask))
	mux.Handle("POST /tutor/c/{id}/delete", protected(tutorH.Delete))

	mux.Handle("GET /topics", protected(topicsH.List))
	mux.Handle("GET /topics/{slug}", protected(topicsH.Show))
	mux.Handle("POST /topics/{slug}/complete", protected(topicsH.ToggleLesson))

	mux.Handle("GET /exercises", protected(exercisesH.List))
	mux.Handle("GET /exercises/new", protected(exercisesH.New))
	mux.Handle("POST /exercises", aiRoute(exercisesH.Create))
	mux.Handle("GET /exercises/{id}", protected(exercisesH.Show))
	mux.Handle("POST /exercises/{id}/complete", protected(exercisesH.Complete))
	mux.Handle("POST /exercises/{id}/hints", aiRoute(exercisesH.Hint))
	mux.Handle("POST /exercises/{id}/solution", aiRoute(exercisesH.Solution))
	mux.Handle("POST /exercises/{id}/submissions", aiRoute(exercisesH.SubmitSolution))

	mux.Handle("GET /review", protected(exercisesH.ReviewPage))
	mux.Handle("POST /review", aiRoute(exercisesH.SubmitReview))
	mux.Handle("GET /review/{id}", protected(exercisesH.ShowReview))

	mux.Handle("GET /quizzes", protected(quizzesH.List))
	mux.Handle("GET /quizzes/new", protected(quizzesH.New))
	mux.Handle("POST /quizzes", aiRoute(quizzesH.Create))
	mux.Handle("GET /quizzes/{id}", protected(quizzesH.Take))
	mux.Handle("POST /quizzes/{id}/attempts", protected(quizzesH.Submit))
	mux.Handle("GET /quizzes/attempts/{id}", protected(quizzesH.Results))

	mux.Handle("GET /progress", protected(progressH.Show))
	mux.Handle("POST /progress/recommendations", aiRoute(progressH.Recommend))

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
		middleware.SecureHeaders(cfg.SecureCookies),
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
