package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jam941/bestaboard/internal/board"
	"github.com/jam941/bestaboard/internal/mode"
)

type Scheduler struct {
	mu       sync.Mutex
	modes    []mode.Mode
	index    int
	interval time.Duration
	pusher   *board.Pusher
	paused   bool
	pinned   bool // when true, only the current mode is shown; no rotation
}

func New(modes []mode.Mode, interval time.Duration, pusher *board.Pusher) *Scheduler {
	return &Scheduler{
		modes:    modes,
		interval: interval,
		pusher:   pusher,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("scheduler starting", "modes", len(s.modes), "interval", s.interval)

	s.tick(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler shutting down")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	s.mu.Lock()
	if s.paused {
		s.mu.Unlock()
		slog.Debug("scheduler paused, skipping tick")
		return
	}

	if !s.pinned {
		s.advance()
	}

	m := s.currentMode()
	s.mu.Unlock()

	if m == nil {
		slog.Warn("no modes registered")
		return
	}

	layout, err := m.Render(ctx)
	if err != nil {
		if errors.Is(err, mode.ErrNoContent) {
			slog.Info("mode has no content, skipping", "mode", m.ID())
			return
		}
		slog.Error("mode render failed", "mode", m.ID(), "error", err)
		return
	}

	slog.Info("pushing mode", "mode", m.ID())
	if err := s.pusher.Push(ctx, layout); err != nil {
		if !errors.Is(err, context.Canceled) {
			slog.Error("pusher push failed", "error", err)
		}
	}
}

func (s *Scheduler) advance() {
	if len(s.modes) == 0 {
		return
	}
	s.index = (s.index + 1) % len(s.modes)
}

func (s *Scheduler) currentMode() mode.Mode {
	if len(s.modes) == 0 {
		return nil
	}
	return s.modes[s.index]
}

func (s *Scheduler) Pause() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = true
	slog.Info("scheduler paused")
}

func (s *Scheduler) Resume() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paused = false
	slog.Info("scheduler resumed")
}

func (s *Scheduler) Skip(ctx context.Context) {
	s.mu.Lock()
	if s.paused {
		s.mu.Unlock()
		return
	}
	s.advance()
	m := s.currentMode()
	s.mu.Unlock()

	if m == nil {
		return
	}

	layout, err := m.Render(ctx)
	if err != nil {
		if !errors.Is(err, mode.ErrNoContent) {
			slog.Error("skip render failed", "mode", m.ID(), "error", err)
		}
		return
	}
	slog.Info("skip: pushing mode", "mode", m.ID())
	if err := s.pusher.Push(ctx, layout); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("skip push failed", "error", err)
	}
}

func (s *Scheduler) ForceMode(ctx context.Context, id string) bool {
	s.mu.Lock()
	idx := -1
	for i, m := range s.modes {
		if m.ID() == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.mu.Unlock()
		return false
	}
	s.index = idx
	s.pinned = true
	m := s.currentMode()
	s.mu.Unlock()

	layout, err := m.Render(ctx)
	if err != nil {
		slog.Error("force render failed", "mode", id, "error", err)
		return true
	}
	slog.Info("force: pushing mode", "mode", id)
	if err := s.pusher.Push(ctx, layout); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("force push failed", "error", err)
	}
	return true
}

func (s *Scheduler) Unpin() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pinned = false
	slog.Info("scheduler unpinned")
}

type Status struct {
	CurrentMode string   `json:"current_mode"`
	Paused      bool     `json:"paused"`
	Pinned      bool     `json:"pinned"`
	ModeIDs     []string `json:"mode_ids"`
}

func (s *Scheduler) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, len(s.modes))
	for i, m := range s.modes {
		ids[i] = m.ID()
	}

	current := ""
	if len(s.modes) > 0 {
		current = s.modes[s.index].ID()
	}

	return Status{
		CurrentMode: current,
		Paused:      s.paused,
		Pinned:      s.pinned,
		ModeIDs:     ids,
	}
}
