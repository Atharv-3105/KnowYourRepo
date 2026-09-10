package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"

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
	dbStore, err := store.NewStore(ctx, cfg.DB.DSN, logger)
	if err != nil{
		return nil, err 
	}

	//Initialize the SideCar
	sidecarClient := sidecar.NewClient(cfg.Extractor.BaseURL)

	//Initialize the GIN client
	router := gin.Default()

	s := &Server{
		cfg:  cfg,
		logger: logger,
		router: router,
		store: dbStore,
		sidecar: sidecarClient,
	}

	s.router.Use(CORSMiddleware(cfg.CORS.AllowedOrigins))
	s.router.Use(RequestIDMiddleware(s.logger))
	s.router.Use(MetricsMiddleware())
	s.registerRoutes()
	return s, nil
}


func (s *Server) registerRoutes() {
	
	//Health Route
	s.router.GET("/health", s.handleHealth)

	//Prometheus metrics scrape endpoint
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	//Repo ingestion route
	repoHandler := NewRepoHandler(s.logger,s.store, s.sidecar)

	// Per-IP token-bucket limiters, attached only to routes that front real
	// LLM/embedding provider cost or heavy background work - not global,
	// since routes like /health or browsing an already-ingested repo don't
	// need this protection. Chat allows a small natural burst (a couple of
	// quick follow-up questions); repo creation/sync is stricter since a
	// single call fans out into a full clone+parse+embed pipeline.
	chatLimiter := NewKeyLimiter(rate.Every(6*time.Second), 5)   // ~10/min, burst 5
	ingestLimiter := NewKeyLimiter(rate.Every(60*time.Second), 2) // ~1/min, burst 2

	s.router.POST("/repos", ingestLimiter.Middleware(), repoHandler.CreateRepo)

	//List ingested repositories
	s.router.GET("/repos", repoHandler.ListRepos)

	//Graph Extraction Route (legacy, unscoped across all repos - kept for backward compat)
	s.router.GET("/graph", repoHandler.GetCallGraph)

	//Bounded, repo-scoped, symbol-anchored call graph
	s.router.GET("/graph/:repoID", repoHandler.GetBoundedCallGraph)

	//Call-Graph Context Route []
	// s.router.GET("/graph/context/:symbol", repoHandler.ExpandSymbolContext)

	//Search EndPoint Route
	s.router.POST("/search", repoHandler.Search)

	//Chat Route
	s.router.POST("/chat", chatLimiter.Middleware(), repoHandler.Chat)

	//Architecture Route
	s.router.GET("/architecture/:repoID", repoHandler.GetArchitecture)

	//Ingestion job status Route
	s.router.GET("/repos/jobs/:id", repoHandler.GetJobStatus)

	//On-demand repo sync check
	s.router.POST("/repos/:id/sync", ingestLimiter.Middleware(), repoHandler.SyncRepo)

	//Raw file content, served from the repo's on-disk clone
	s.router.GET("/files/:repoID", repoHandler.GetFileContent)

	//Full, searchable symbol index for a repo
	s.router.GET("/symbols/:repoID", repoHandler.ListSymbols)
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


