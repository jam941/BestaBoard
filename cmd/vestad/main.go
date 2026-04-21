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
	"github.com/jam941/bestaboard/internal/config"
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

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if os.Getenv("DEMO") == "true" {
		runDemo(ctx, cancel, sigCh, client)
	} else {
		runProd(ctx, cancel, sigCh, client)
	}
}

func runDemo(ctx context.Context, cancel context.CancelFunc, sigCh <-chan os.Signal, client *vestaboard.Client) {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	slog.Info("demo mode: config loaded", "modes", cfg.Modes, "notes", len(cfg.Notes))

	pusher := board.NewPusher(client)

	modes := buildDemoModes(cfg)
	h := hub.New()
	sched := scheduler.New(modes, cfg.RotationInterval.Duration, pusher, h)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	httpServer := &http.Server{Addr: ":8080", Handler: mux}

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
		slog.Info("http server listening (demo)", "addr", httpServer.Addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http server error", "error", err)
		}
	}()

	sched.Start(ctx)
	pusher.Stop()

	time.Sleep(100 * time.Millisecond)
	slog.Info("shutdown complete")
}

func buildDemoModes(cfg *config.Config) []mode.Mode {
	all := map[string]mode.Mode{
		"clock": mode.NewClockMode(func() string { return cfg.Weather.Timezone }),
		"static": mode.NewStaticMode(func() string { return cfg.StaticText }),
		"weather": mode.NewWeatherMode(func() mode.WeatherConfig {
			return mode.WeatherConfig{
				Latitude:  cfg.Weather.Latitude,
				Longitude: cfg.Weather.Longitude,
				Timezone:  cfg.Weather.Timezone,
				Units:     cfg.Weather.Units,
			}
		}),
		"notes": buildDemoNoteMode(cfg),
	}

	enabled := cfg.Modes
	if len(enabled) == 0 {
		enabled = []string{"clock", "weather", "notes", "static"}
	}

	var modes []mode.Mode
	for _, id := range enabled {
		if m, ok := all[id]; ok {
			modes = append(modes, m)
		} else {
			slog.Warn("demo: unknown mode in config, skipping", "mode", id)
		}
	}
	return modes
}

func buildDemoNoteMode(cfg *config.Config) *mode.DemoNoteMode {
	notes := make([]mode.DemoNote, len(cfg.Notes))
	for i, n := range cfg.Notes {
		d := n.Duration.Duration
		if d <= 0 {
			d = cfg.NoteDuration.Duration
		}
		notes[i] = mode.DemoNote{Text: n.Text, Duration: d}
	}
	return mode.NewDemoNoteMode(notes)
}

func runProd(ctx context.Context, cancel context.CancelFunc, sigCh <-chan os.Signal, client *vestaboard.Client) {
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
