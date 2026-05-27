package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/atharva-3105/KnowYourRepo/internal/config"
	"github.com/atharva-3105/KnowYourRepo/internal/sidecar"
	"github.com/atharva-3105/KnowYourRepo/internal/store"
)

type Server struct {
	cfg *config.Config
	logger *slog.Logger
	router *gin.Engine
	store  *store.Store
	sidecar *sidecar.Client
}

func NewServer(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	ctx := context.Background()
	//Initialize the SQLite Store
	dbStore, err := store.NewStore(ctx, "knowyourrepo.db", logger)
	if err != nil{
		return nil, err 
	}

	//Initialize the SideCar
	sidecarClient := sidecar.NewClient("http://localhost:8000")

	//Initialize the GIN client
	router := gin.Default()

	s := &Server{
		cfg:  cfg,
		logger: logger,
		router: router,
		store: dbStore,
		sidecar: sidecarClient,
	}

	s.registerRoutes()
	return s, nil
}


func (s *Server) registerRoutes() {
	
	//Health Route
	s.router.GET("/health", s.handleHealth)

	//Repo ingestion route
	repoHandler := NewRepoHandler(s.logger,s.store, s.sidecar)

	s.router.POST("/repos", repoHandler.CreateRepo)

	//Graph Extraction Route
	s.router.GET("/graph", repoHandler.GetCallGraph)
}

func (s *Server) Start() error {

	s.logger.Info("starting api server", "addr", s.cfg.Server.Addr())
	
	return s.router.Run(s.cfg.Server.Addr())
}


func (s *Server) handleHealth(c *gin.Context) {

	c.JSON(
		http.StatusOK,
		gin.H{
			"status": "ok",
			"service": "api",
		},
	)
}


