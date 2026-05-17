package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/atharva-3105/KnowYourRepo/internal/config"
)

type Server struct {
	cfg *config.Config
	logger *slog.Logger
	mux    *http.ServeMux
}

func NewServer(cfg *config.Config, logger *slog.Logger) *Server {
	s := &Server{
		cfg: cfg, 
		logger: logger,
		mux:	http.NewServeMux(),
	}

	s.registerRoutes()
	return s
}


func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
}

func (s *Server) Start() error {
	return http.ListenAndServe(s.cfg.Server.Addr(), s.mux)
}


func (s *Server) handleHealth( w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":"ok", 
		"service": "api",
	})
}