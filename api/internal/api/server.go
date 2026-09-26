package api

import (
	"context"
	"errors"
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
	cfg         *config.Config
	logger      *slog.Logger
	router      *gin.Engine
	store       *store.Store
	sidecar     *sidecar.Client
	repoHandler *RepoHandler
	httpServer  *http.Server
}

func NewServer(cfg *config.Config, logger *slog.Logger) (*Server, error) {
	ctx := context.Background()

	dbStore, err := store.NewStore(ctx, cfg.DB.DSN, logger)
	if err != nil {
		return nil, err
	}

	sidecarClient := sidecar.NewClient(cfg.Extractor.BaseURL).WithAPIKey(cfg.Extractor.APIKey)
	if cfg.Extractor.APIKey == "" {
		logger.Warn("sidecar_api_key_not_set", "detail", "SIDECAR_API_KEY is empty - fine locally, but a publicly reachable sidecar must require a shared secret")
	}

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Never let gin parse X-Forwarded-For itself: with every proxy trusted
	// (its default) it returns the client-controlled leftmost entry, which
	// would let a caller mint a fresh rate-limit bucket per request. Client
	// IPs for rate limiting come from ResolveClientIP instead.
	if err := router.SetTrustedProxies(nil); err != nil {
		return nil, err
	}

	s := &Server{
		cfg:     cfg,
		logger:  logger,
		router:  router,
		store:   dbStore,
		sidecar: sidecarClient,
	}

	s.router.Use(SecurityHeadersMiddleware())
	s.router.Use(BodyLimitMiddleware(cfg.Limits.MaxRequestBodyBytes))
	s.router.Use(CORSMiddleware(cfg.CORS.AllowedOrigins))
	s.router.Use(RequestIDMiddleware(s.logger))
	s.router.Use(MetricsMiddleware())
	s.registerRoutes()

	s.httpServer = &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           s.router,
		ReadHeaderTimeout: 10 * time.Second,
		// Chat answers wait on an LLM call (and possibly a queued embedding
		// call behind a free-tier rate limit) - generous, but bounded.
		WriteTimeout: 3 * time.Minute,
		IdleTimeout:  2 * time.Minute,
	}

	return s, nil
}

func (s *Server) clientKey(c *gin.Context) string {
	return ResolveClientIP(c.Request, s.cfg.Proxy.TrustedHops, s.cfg.Proxy.ClientIPHeader)
}

func (s *Server) registerRoutes() {

	s.router.GET("/health", s.handleHealth)

	//Prometheus metrics scrape endpoint
	s.router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	repoHandler := NewRepoHandler(s.cfg, s.logger, s.store, s.sidecar)
	s.repoHandler = repoHandler

	// Per-IP token-bucket limiters, attached only to routes that front real
	// LLM/embedding provider cost or heavy background work - not global,
	// since routes like /health or browsing an already-ingested repo don't
	// need this protection. Chat allows a small natural burst (a couple of
	// quick follow-up questions); repo creation/sync is stricter since a
	// single call fans out into a full clone+parse+embed pipeline; search
	// embeds its query, so it's limited too.
	chatLimiter := NewKeyLimiter(rate.Every(6*time.Second), 5).WithKeyFunc(s.clientKey)    // ~10/min, burst 5
	ingestLimiter := NewKeyLimiter(rate.Every(60*time.Second), 2).WithKeyFunc(s.clientKey) // ~1/min, burst 2
	searchLimiter := NewKeyLimiter(rate.Every(4*time.Second), 4).WithKeyFunc(s.clientKey)  // ~15/min, burst 4

	s.router.POST("/repos", ingestLimiter.Middleware(), repoHandler.CreateRepo)

	//List ingested repositories
	s.router.GET("/repos", repoHandler.ListRepos)

	//Bounded, repo-scoped, symbol-anchored call graph
	s.router.GET("/graph/:repoID", repoHandler.GetBoundedCallGraph)

	//Semantic search (command palette)
	s.router.POST("/search", searchLimiter.Middleware(), repoHandler.Search)

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

// Start serves HTTP until ctx is cancelled (SIGINT/SIGTERM from main), then
// shuts down gracefully: stop accepting connections, let in-flight requests
// finish, then give in-flight ingestion jobs until the deadline before
// cancelling them (they're recorded as pending and resume on next start).
func (s *Server) Start(ctx context.Context, shutdownTimeout time.Duration) error {

	// Jobs interrupted by a previous shutdown/crash resume now.
	s.repoHandler.ResumeUnfinishedJobs(ctx)

	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("starting api server", "addr", s.cfg.Server.Addr(), "env", s.cfg.Env)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err, ok := <-errCh:
		if ok && err != nil {
			return err
		}
	case <-ctx.Done():
	}

	s.logger.Info("shutdown_started", "timeout", shutdownTimeout.String())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("http_shutdown_failed", "error", err)
	}

	if err := s.repoHandler.Shutdown(shutdownCtx); err != nil {
		s.logger.Warn("ingestion_shutdown_deadline_exceeded", "error", err)
	}

	if err := s.store.Close(); err != nil {
		s.logger.Error("db_close_failed", "error", err)
	}

	s.logger.Info("shutdown_complete")
	return nil
}

// handleHealth is a readiness check: it verifies the database is reachable,
// since a server that can't reach Postgres can't serve any real request.
func (s *Server) handleHealth(c *gin.Context) {

	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := s.store.DB().PingContext(ctx); err != nil {
		s.logger.Error("health_db_unreachable", "error", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "service": "api", "database": "unreachable"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "api"})
}
