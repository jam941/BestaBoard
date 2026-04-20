package httpapi

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"time"

	"github.com/jam941/bestaboard/internal/hub"
	"github.com/jam941/bestaboard/internal/mode"
	"github.com/jam941/bestaboard/internal/scheduler"
	"github.com/jam941/bestaboard/internal/store"
)

type Server struct {
	router *chi.Mux
	sched  *scheduler.Scheduler
	hub    *hub.Hub
	store  *store.Store
}

func New(sched *scheduler.Scheduler, h *hub.Hub, st *store.Store) *Server {
	s := &Server{
		sched: sched,
		hub:   h,
		store: st,
	}

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(logRequests)

	// Public — no auth.
	r.Get("/health", s.handleHealth)
	r.Get("/events", s.handleEvents)
	r.Post("/login", s.handleLogin)

	// Protected — session token required.
	r.Group(func(r chi.Router) {
		r.Use(s.bearerAuth)
		r.Post("/logout", s.handleLogout)
		r.Get("/status", s.handleStatus)
		r.HandleFunc("/pause", s.handlePause)
		r.HandleFunc("/resume", s.handleResume)
		r.HandleFunc("/skip", s.handleSkip)
		r.HandleFunc("/force/{modeID}", s.handleForce)
		r.HandleFunc("/unpin", s.handleUnpin)
		r.HandleFunc("/modes/{modeID}/enable", s.handleEnableMode)
		r.HandleFunc("/modes/{modeID}/disable", s.handleDisableMode)
		r.Get("/modes/{modeID}/preview", s.handlePreviewMode)
		r.Post("/notes", s.handleCreateNote)
		r.Get("/notes", s.handleListNotes)
		r.Delete("/notes/{noteID}", s.handleDismissNote)
		r.Post("/users", s.handleCreateUser)
		r.Get("/preferences", s.handleGetPreferences)
		r.Patch("/preferences", s.handleUpdatePreferences)
	})

	s.router = r
	return s
}

func (s *Server) Handler() http.Handler {
	return s.router
}


func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Info("http request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) bearerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		token, found := strings.CutPrefix(auth, "Bearer ")
		if !found || !s.store.ValidateSession(token) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	user, err := s.store.AuthenticateUser(body.Username, body.Password)
	if err != nil {
		slog.Error("authenticate user failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	token, err := s.store.CreateSession(user.ID)
	if err != nil {
		slog.Error("create session failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if err := s.store.DeleteSession(token); err != nil {
		slog.Error("delete session failed", "error", err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}


func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.sched.Status())
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	s.sched.Pause()
	writeJSON(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	s.sched.Resume()
	writeJSON(w, http.StatusOK, map[string]string{"status": "resumed"})
}

func (s *Server) handleSkip(w http.ResponseWriter, r *http.Request) {
	s.sched.Skip(r.Context())
	writeJSON(w, http.StatusOK, map[string]string{"status": "skipped"})
}

func (s *Server) handleForce(w http.ResponseWriter, r *http.Request) {
	modeID := chi.URLParam(r, "modeID")
	if !s.sched.ForceMode(r.Context(), modeID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mode not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "forced", "mode": modeID})
}

func (s *Server) handleUnpin(w http.ResponseWriter, r *http.Request) {
	s.sched.Unpin()
	writeJSON(w, http.StatusOK, map[string]string{"status": "unpinned"})
}

func (s *Server) handleEnableMode(w http.ResponseWriter, r *http.Request) {
	modeID := chi.URLParam(r, "modeID")
	if !s.sched.EnableMode(modeID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mode not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "enabled", "mode": modeID})
}

func (s *Server) handleDisableMode(w http.ResponseWriter, r *http.Request) {
	modeID := chi.URLParam(r, "modeID")
	if !s.sched.DisableMode(modeID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mode not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "disabled", "mode": modeID})
}

func (s *Server) handlePreviewMode(w http.ResponseWriter, r *http.Request) {
	modeID := chi.URLParam(r, "modeID")
	m, ok := s.sched.GetMode(modeID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "mode not found"})
		return
	}
	layout, err := m.Render(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":   modeID,
		"text": mode.LayoutToText(layout),
	})
}


// handleEvents streams Server-Sent Events to the client. It sends the
// current status immediately on connect, then pushes an event whenever the
// scheduler broadcasts a state change. The endpoint is intentionally public
// (read-only status data) so EventSource can connect without custom headers.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx/proxy buffering

	// Send the current state immediately so the client doesn't wait.
	if data, err := json.Marshal(s.sched.Status()); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	ch := s.hub.Subscribe()
	defer s.hub.Unsubscribe(ch)

	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text            string `json:"text"`
		DurationMinutes int    `json:"duration_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if strings.TrimSpace(body.Text) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text is required"})
		return
	}

	duration := 15 * time.Minute
	if prefs, err := s.store.GetPreferences(); err == nil {
		if d, err := time.ParseDuration(prefs.NoteDuration); err == nil && d > 0 {
			duration = d
		}
	}
	if body.DurationMinutes > 0 {
		duration = time.Duration(body.DurationMinutes) * time.Minute
	}

	note, err := s.store.CreateNote(body.Text, duration)
	if err != nil {
		slog.Error("create note failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create note"})
		return
	}

	s.sched.ForceMode(r.Context(), "notes")

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":         note.ID,
		"text":       note.Text,
		"expires_at": note.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleListNotes(w http.ResponseWriter, r *http.Request) {
	notes, err := s.store.RecentNotes(10)
	if err != nil {
		slog.Error("list notes failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list notes"})
		return
	}

	type noteJSON struct {
		ID          int64   `json:"id"`
		Text        string  `json:"text"`
		CreatedAt   string  `json:"created_at"`
		ExpiresAt   string  `json:"expires_at"`
		DismissedAt *string `json:"dismissed_at"`
		Active      bool    `json:"active"`
	}

	out := make([]noteJSON, 0, len(notes))
	for _, n := range notes {
		nj := noteJSON{
			ID:        n.ID,
			Text:      n.Text,
			CreatedAt: n.CreatedAt.UTC().Format(time.RFC3339),
			ExpiresAt: n.ExpiresAt.UTC().Format(time.RFC3339),
			Active:    n.Active(),
		}
		if n.DismissedAt != nil {
			s := n.DismissedAt.UTC().Format(time.RFC3339)
			nj.DismissedAt = &s
		}
		out = append(out, nj)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDismissNote(w http.ResponseWriter, r *http.Request) {
	var id int64
	if _, err := fmt.Sscan(chi.URLParam(r, "noteID"), &id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid note ID"})
		return
	}
	if err := s.store.DismissNote(id); err != nil {
		slog.Error("dismiss note failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to dismiss note"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "dismissed"})
}

func (s *Server) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	prefs, err := s.store.GetPreferences()
	if err != nil {
		slog.Error("get preferences failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load preferences"})
		return
	}
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	prefs, err := s.store.GetPreferences()
	if err != nil {
		slog.Error("get preferences failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load preferences"})
		return
	}
	if err := json.NewDecoder(r.Body).Decode(prefs); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	interval, err := time.ParseDuration(prefs.RotationInterval)
	if err != nil || interval <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid rotation_interval"})
		return
	}
	if _, err := time.ParseDuration(prefs.NoteDuration); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid note_duration"})
		return
	}
	if err := s.store.UpdatePreferences(prefs); err != nil {
		slog.Error("update preferences failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save preferences"})
		return
	}
	s.sched.SetInterval(interval)
	writeJSON(w, http.StatusOK, prefs)
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if strings.TrimSpace(body.Username) == "" || body.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password are required"})
		return
	}
	if err := s.store.CreateUser(body.Username, body.Password); err != nil {
		slog.Error("create user failed", "error", err)
		writeJSON(w, http.StatusConflict, map[string]string{"error": "username already exists"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"username": body.Username})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to write JSON response", "error", err)
	}
}
