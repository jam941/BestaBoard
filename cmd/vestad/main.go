package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jam941/Vestaboard-Golang/vestaboard"
	"github.com/jam941/bestaboard/internal/board"
	"github.com/jam941/bestaboard/internal/config"
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

	pusher := board.NewPusher(client)


	modes := []mode.Mode{
		mode.NewClockMode(),
		mode.NewStaticMode(cfg.StaticText),
	}

	sched := scheduler.New(modes, cfg.RotationInterval.Duration, pusher)

	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	sched.Start(ctx)
	pusher.Stop()

	time.Sleep(100 * time.Millisecond)
	slog.Info("shutdown complete")
}
