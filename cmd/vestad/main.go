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
	"github.com/jam941/bestaboard/internal/httpapi"
	"github.com/jam941/bestaboard/internal/mode"
	"github.com/jam941/bestaboard/internal/scheduler"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	slog.Info("config loaded",
		"rotation_interval", cfg.RotationInterval.Duration,
		"static_text", cfg.StaticText,
	)

	token := os.Getenv("VBOARD_TOKEN")
	if token == "" {
		slog.Error("VBOARD_TOKEN env var is required")
		os.Exit(1)
	}
	client := vestaboard.NewNote(token)

	authToken := os.Getenv("AUTH_TOKEN")
	if authToken == "" {
		slog.Warn("AUTH_TOKEN not set — running without authentication")
	}

	pusher := board.NewPusher(client)

	modes := []mode.Mode{
		mode.NewClockMode(),
		mode.NewStaticMode(cfg.StaticText),
	}

	sched := scheduler.New(modes, cfg.RotationInterval.Duration, pusher)

	// HTTP server — runs alongside the scheduler.
	apiServer := httpapi.New(sched, authToken)
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
