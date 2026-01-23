package main

import (
	"coderefinery/graph"
	"coderefinery/internal/adapters/indexer"
	"context"

	repoPG "coderefinery/internal/adapters/repository/postgres"
	storagePG "coderefinery/internal/adapters/storage/postgres"

	"coderefinery/internal/config"
	"coderefinery/internal/core/services"

	"coderefinery/internal/infrastructure/auth"
	"coderefinery/internal/infrastructure/cache"
	"coderefinery/internal/infrastructure/llm"
	"coderefinery/internal/infrastructure/logging"
	"coderefinery/internal/infrastructure/metrics"
	"coderefinery/internal/infrastructure/middleware"
	"coderefinery/internal/infrastructure/tracing"
	"coderefinery/internal/search"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/rs/zerolog/log"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// 1. Config laden
	cfg, err := config.LoadConfig(".")
	if err != nil {
		// Hier nutzen wir noch panic oder println, da Logger noch nicht konfiguriert
		panic("Critical: Config validation failed: " + err.Error())
	}

	cacheService, err := cache.NewHybridCache(cfg.Cache)
    if err != nil {
         log.Warn().Err(err).Msg("Failed to init cache, continuing without caching")
    }

	// 2. Logger initialisieren
	logging.Init(cfg.Observability.Logging)

	log.Info().Msgf("Starting CodeRefinery in %s mode", cfg.Environment)

	var metricService *metrics.Metrics
		if cfg.Observability.Metrics.Enabled {
			metricService = metrics.NewMetrics("coderefinery")
		}

		shutdownTracer, err := tracing.InitTracer(cfg.Observability.Tracing)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to init tracer")
    }
    defer func() {
        if err := shutdownTracer(context.Background()); err != nil {
            log.Error().Err(err).Msg("Error shutting down tracer")
        }
    }()
    log.Info().Bool("enabled", cfg.Observability.Tracing.Enabled).Msg("Tracing initialized")

	// 3. Datenbank öffnen
	log.Info().Str("driver", cfg.Database.Driver).Str("source", cfg.Database.Source).Msg("Connecting to database")
	db, err := sqlx.Connect("pgx", cfg.Database.Source)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open db")
	}

	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)

	defer db.Close()

	// 4. Adapter initialisieren
	repoStore := storagePG.NewRepoStore(db)
	chunkRepo := repoPG.NewChunkRepository(db)

	// Embedder initialisieren
	embedder, err := llm.NewOllamaEmbedder(cfg.LLM)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create embedder")
	}

	// Indexer
	idx, err := indexer.NewIndexer(cfg.Indexer, embedder, db)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to init indexer")
	}

	// 5. Service Layer
	repoService := services.NewRepositoryService(repoStore, idx)
	searcher := search.NewSearcher(chunkRepo, embedder, cacheService)

	userStore := storagePG.NewUserStore(db)
	jwtService := auth.NewJWTService(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry)
	authService := services.NewAuthService(userStore, jwtService)

	// 6. GraphQL Server Setup
	// Gin Mode setzen (Release Mode unterdrückt Debug-Output von Gin selbst)
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	if metricService != nil {
		r.Use(metricService.GinMiddleware())

		// Endpoint für Prometheus Scraper bereitstellen
		r.GET(cfg.Observability.Metrics.Path, gin.WrapH(promhttp.Handler()))
		log.Info().Str("path", cfg.Observability.Metrics.Path).Msg("Metrics endpoint enabled")
	}

	r.Static("/static", "./web/static")
	r.GET("/", func(c *gin.Context) { c.File("./web/index.html") })

	r.GET("/playground", func(c *gin.Context) {
		playground.Handler("GraphQL", "/query").ServeHTTP(c.Writer, c.Request)
	})

	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{
		Resolvers: &graph.Resolver{
			RepoService: repoService,
			Searcher:    searcher,
			AuthService: authService,
			Embedder:    embedder,
			Config:      cfg,
		},
	}))

	r.POST("/query", middleware.AuthMiddleware(jwtService), func(c *gin.Context) {
		srv.ServeHTTP(c.Writer, c.Request)
	})

	r.GET("/health", func(c *gin.Context) {
		// Wir könnten hier auch kurz DB und Redis pingen,
		// aber für Liveness Probes reicht oft ein einfaches 200 OK.
		c.JSON(200, gin.H{
			"status": "up",
			"env":    cfg.Environment,
		})
	})

	log.Info().Str("port", cfg.Server.Port).Msg("Playground ready")
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}
