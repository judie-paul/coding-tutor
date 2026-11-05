// Command web runs the AI Programming Tutor web server.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"codingtutor/internal/config"
	"codingtutor/internal/database"
	"codingtutor/migrations"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "apply database migrations and exit")
	flag.Parse()

	if err := run(*migrateOnly); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(migrateOnly bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := newLogger(cfg)
	slog.SetDefault(logger)

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx := context.Background()
	applied, err := database.Migrate(ctx, db, migrations.FS)
	if err != nil {
		return err
	}
	logger.Info("migrations complete", "applied", applied, "db", cfg.DBPath)
	if migrateOnly {
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      cfg.AITimeout + 30*time.Second, // AI requests can be slow
		IdleTimeout:       120 * time.Second,
	}
	return serve(srv, logger)
}

// serve runs srv until SIGINT/SIGTERM, then shuts down gracefully.
func serve(srv *http.Server, logger *slog.Logger) error {
	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case sig := <-stop:
		logger.Info("shutting down", "signal", sig.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func newLogger(cfg config.Config) *slog.Logger {
	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: level}
	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
