package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jam941/Vestaboard-Golang/vestaboard"
	"github.com/jam941/bestaboard/internal/board"
	"github.com/jam941/bestaboard/internal/hub"
	"github.com/jam941/bestaboard/internal/httpapi"
	"github.com/jam941/bestaboard/internal/mode"
	"github.com/jam941/bestaboard/internal/scheduler"
	"github.com/jam941/bestaboard/internal/store"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	token := os.Getenv("VBOARD_TOKEN")
	if token == "" {
		slog.Error("VBOARD_TOKEN env var is required")
		os.Exit(1)
	}
	client := vestaboard.NewNote(token)

	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		slog.Error("DATABASE_URL env var is required")
		os.Exit(1)
	}
	db, err := store.Open(connStr)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("database opened")

	adminUser := os.Getenv("ADMIN_USER")
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminUser != "" && adminPass != "" {
		if err := db.SeedAdminIfEmpty(adminUser, adminPass); err != nil {
			slog.Error("failed to seed admin user", "error", err)
			os.Exit(1)
		}
	}

	prefs, err := db.GetPreferences()
	if err != nil {
		slog.Error("failed to load preferences", "error", err)
		os.Exit(1)
	}

	rotationInterval, err := time.ParseDuration(prefs.RotationInterval)
	if err != nil || rotationInterval <= 0 {
		rotationInterval = time.Minute
	}

	pusher := board.NewPusher(client)
	h := hub.New()

	getPrefs := func() *store.Preferences {
		p, err := db.GetPreferences()
		if err != nil {
			slog.Warn("failed to read preferences, using last known", "error", err)
			return prefs
		}
		return p
	}

	modes := []mode.Mode{
		mode.NewNoteMode(db),
		mode.NewClockMode(func() string { return getPrefs().WeatherTimezone }),
		mode.NewStaticMode(func() string { return getPrefs().StaticText }),
		mode.NewWeatherMode(func() mode.WeatherConfig {
			p := getPrefs()
			return mode.WeatherConfig{
				Latitude:  p.WeatherLatitude,
				Longitude: p.WeatherLongitude,
				Timezone:  p.WeatherTimezone,
				Units:     p.WeatherUnits,
			}
		}),
	}

	sched := scheduler.New(modes, rotationInterval, pusher, h)

	apiServer := httpapi.New(sched, h, db)
	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: apiServer.Handler(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		cancel()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("http server shutdown error", "error", err)
		}
	}()

	go func() {
		slog.Info("http server listening", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
		}
	}()

	sched.Start(ctx)
	pusher.Stop()

	time.Sleep(100 * time.Millisecond)
	slog.Info("shutdown complete")
}
