package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/jam941/bestaboard/internal/board"
	"github.com/jam941/bestaboard/internal/hub"
	"github.com/jam941/bestaboard/internal/mode"
)

type Scheduler struct {
	mu       sync.Mutex
	modes    []mode.Mode
	enabled  map[string]bool
	index    int
	interval time.Duration
	pusher   *board.Pusher
	hub      *hub.Hub
	paused   bool
	pinned   bool // when true, only the current mode is shown; no rotation
	resetCh  chan struct{}
}

func New(modes []mode.Mode, interval time.Duration, pusher *board.Pusher, h *hub.Hub) *Scheduler {
	enabled := make(map[string]bool, len(modes))
	for _, m := range modes {
		enabled[m.ID()] = true
	}
	return &Scheduler{
		modes:    modes,
		enabled:  enabled,
		interval: interval,
		pusher:   pusher,
		hub:      h,
		resetCh:  make(chan struct{}, 1),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	slog.Info("scheduler starting", "modes", len(s.modes), "interval", s.interval)

	s.tick(ctx)

	ticker := time.NewTicker(s.currentDuration())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("scheduler shutting down")
			return
		case <-s.resetCh:
			ticker.Reset(s.currentDuration())
		case <-ticker.C:
			s.tick(ctx)
			ticker.Reset(s.currentDuration())
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
	if m != nil && !s.enabled[m.ID()] {
		s.mu.Unlock()
		slog.Debug("current mode disabled, skipping tick", "mode", m.ID())
		return
	}
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
		return
	}
	s.broadcast()
}

// advance moves to the next enabled mode. If all modes are disabled it stays
// on the current index. Must be called with s.mu held.
func (s *Scheduler) advance() {
	n := len(s.modes)
	if n == 0 {
		return
	}
	for i := 1; i <= n; i++ {
		next := (s.index + i) % n
		if s.enabled[s.modes[next].ID()] {
			s.index = next
			return
		}
	}
	// All modes disabled — stay put.
}

// currentMode returns the mode at the current index. Must be called with s.mu held.
func (s *Scheduler) currentMode() mode.Mode {
	if len(s.modes) == 0 {
		return nil
	}
	return s.modes[s.index]
}

// currentDuration returns the rotation duration for the current mode.
// If the mode implements DurationProvider and returns a non-zero value,
// that is used; otherwise the global interval is returned.
func (s *Scheduler) currentDuration() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.modes) == 0 {
		return s.interval
	}
	m := s.modes[s.index]
	if dp, ok := m.(mode.DurationProvider); ok {
		if d := dp.Duration(); d > 0 {
			return d
		}
	}
	return s.interval
}

func (s *Scheduler) Pause() {
	s.mu.Lock()
	s.paused = true
	s.mu.Unlock()
	slog.Info("scheduler paused")
	s.broadcast()
}

func (s *Scheduler) Resume() {
	s.mu.Lock()
	s.paused = false
	s.mu.Unlock()
	slog.Info("scheduler resumed")
	s.broadcast()
}

func (s *Scheduler) Skip(ctx context.Context) {
	slog.Info("skip: received")
	s.mu.Lock()
	if s.paused {
		slog.Info("skip: ignored — scheduler is paused")
		s.mu.Unlock()
		return
	}
	s.advance()
	m := s.currentMode()
	slog.Info("skip: advanced to mode", "mode", m.ID())
	s.mu.Unlock()

	if m == nil {
		slog.Warn("skip: no modes registered")
		return
	}

	slog.Info("skip: rendering mode", "mode", m.ID())
	layout, err := m.Render(ctx)
	if err != nil {
		if errors.Is(err, mode.ErrNoContent) {
			slog.Info("skip: mode returned no content", "mode", m.ID())
		} else {
			slog.Error("skip: render failed", "mode", m.ID(), "error", err)
		}
		return
	}

	slog.Info("skip: sending to pusher", "mode", m.ID())
	if err := s.pusher.Push(ctx, layout); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("skip: push failed", "error", err)
		return
	}
	slog.Info("skip: done, resetting interval")
	s.broadcast()
	s.resetInterval()
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
		return true
	}
	s.broadcast()
	s.resetInterval()
	return true
}

// EnableMode enables a mode by ID. Returns false if the ID is not found.
func (s *Scheduler) EnableMode(id string) bool {
	s.mu.Lock()
	found := false
	for _, m := range s.modes {
		if m.ID() == id {
			found = true
			break
		}
	}
	if found {
		s.enabled[id] = true
	}
	s.mu.Unlock()
	if found {
		slog.Info("mode enabled", "mode", id)
		s.broadcast()
	}
	return found
}

// DisableMode disables a mode by ID. Returns false if the ID is not found.
func (s *Scheduler) DisableMode(id string) bool {
	s.mu.Lock()
	found := false
	for _, m := range s.modes {
		if m.ID() == id {
			found = true
			break
		}
	}
	if found {
		s.enabled[id] = false
	}
	s.mu.Unlock()
	if found {
		slog.Info("mode disabled", "mode", id)
		s.broadcast()
	}
	return found
}

// resetInterval signals Start to reset the ticker. Non-blocking — if a reset
// is already pending it's a no-op (channel is buffered size 1).
func (s *Scheduler) resetInterval() {
	select {
	case s.resetCh <- struct{}{}:
	default:
	}
}

func (s *Scheduler) Unpin() {
	s.mu.Lock()
	s.pinned = false
	s.mu.Unlock()
	slog.Info("scheduler unpinned")
	s.broadcast()
}

// broadcast sends the current status to all SSE subscribers.
// Safe to call with or without the mutex held (reads its own lock internally).
func (s *Scheduler) broadcast() {
	if s.hub == nil {
		return
	}
	s.hub.Broadcast(s.Status())
}

// ModeInfo describes a single registered mode and its runtime enabled state.
type ModeInfo struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

type Status struct {
	CurrentMode string     `json:"current_mode"`
	Paused      bool       `json:"paused"`
	Pinned      bool       `json:"pinned"`
	Modes       []ModeInfo `json:"modes"`
}

func (s *Scheduler) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	modes := make([]ModeInfo, len(s.modes))
	for i, m := range s.modes {
		modes[i] = ModeInfo{
			ID:      m.ID(),
			Enabled: s.enabled[m.ID()],
		}
	}

	current := ""
	if len(s.modes) > 0 {
		current = s.modes[s.index].ID()
	}

	return Status{
		CurrentMode: current,
		Paused:      s.paused,
		Pinned:      s.pinned,
		Modes:       modes,
	}
}
