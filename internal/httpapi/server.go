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
	"github.com/jam941/bestaboard/internal/hub"
	"github.com/jam941/bestaboard/internal/scheduler"
)

type Server struct {
	router    *chi.Mux
	sched     *scheduler.Scheduler
	hub       *hub.Hub
	authToken string
}

func New(sched *scheduler.Scheduler, authToken string, h *hub.Hub) *Server {
	s := &Server{
		sched:     sched,
		hub:       h,
		authToken: authToken,
	}

	r := chi.NewRouter()
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
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
	r.Get("/events", s.handleEvents) // SSE stream — read-only, no auth needed

	// Protected — bearer token required.
	r.Group(func(r chi.Router) {
		r.Use(s.bearerAuth)
		r.Get("/status", s.handleStatus)
		r.HandleFunc("/pause", s.handlePause)
		r.HandleFunc("/resume", s.handleResume)
		r.HandleFunc("/skip", s.handleSkip)
		r.HandleFunc("/force/{modeID}", s.handleForce)
		r.HandleFunc("/unpin", s.handleUnpin)
		r.HandleFunc("/modes/{modeID}/enable", s.handleEnableMode)
		r.HandleFunc("/modes/{modeID}/disable", s.handleDisableMode)
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
		// No token configured → auth disabled (local dev mode).
		if s.authToken == "" {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		token, found := strings.CutPrefix(auth, "Bearer ")
		if !found || token != s.authToken {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to write JSON response", "error", err)
	}
}
