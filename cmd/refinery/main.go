package main

import (
	"context"
	"net/http"

	"coderefinery/graph"
	"coderefinery/internal/adapters/indexer"
	"coderefinery/internal/adapters/vectordb"

	storagePG "coderefinery/internal/adapters/storage/postgres"
	storageWeaviate "coderefinery/internal/adapters/storage/weaviate"

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
	"github.com/weaviate/weaviate-go-client/v4/weaviate"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	// 1. Config laden
	cfg, err := config.LoadConfig(".")
	if err != nil {
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

	// 3. Relationale Datenbank (Postgres)
	log.Info().Str("source", cfg.Database.Source).Msg("Connecting to auth database (Postgres)")
	db, err := sqlx.Connect("pgx", cfg.Database.Source)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open auth db")
	}
	db.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.Database.ConnMaxLifetime)
	defer db.Close()

	// 4. Vector Datenbank (Weaviate) Setup

	// A) Weaviate Client initialisieren
	log.Info().Str("host", cfg.VectorDB.Host).Msg("Connecting to Weaviate")

	wCfg := weaviate.Config{
		Host:   cfg.VectorDB.Host,
		Scheme: cfg.VectorDB.Scheme,
		ConnectionClient: &http.Client{
			Timeout: cfg.VectorDB.Timeout,
		},
	}

	weaviateClient, err := weaviate.NewClient(wCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create Weaviate client")
	}

	// B) Vector Store initialisieren (für Code Chunks)
	vectorStore, err := vectordb.NewWeaviateVectorStore(weaviateClient, cfg.VectorDB.IndexName)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to init vector store")
	}

	// 5. Adapter & Services initialisieren

	// Stores
	// RepoStore nutzt jetzt Weaviate für Metadaten
	repoStore, err := storageWeaviate.NewRepoStore(weaviateClient)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to init weaviate repo store")
	}

	// UserStore bleibt bei Postgres (Auth)
	userStore := storagePG.NewUserStore(db)

	// Embedder (Ollama)
	embedder, err := llm.NewOllamaEmbedder(cfg.LLM)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create embedder")
	}

	// Indexer
	idx, err := indexer.NewIndexer(cfg.Indexer, embedder, vectorStore)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to init indexer")
	}

	// Core Services
	repoService := services.NewRepositoryService(repoStore, idx)

	// Searcher
	searcher := search.NewSearcher(vectorStore, embedder, cacheService)

	// Auth
	jwtService := auth.NewJWTService(cfg.Auth.JWTSecret, cfg.Auth.JWTExpiry)
	authService := services.NewAuthService(userStore, jwtService)

	// 6. GraphQL Server Setup
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	if metricService != nil {
		r.Use(metricService.GinMiddleware())
		r.GET(cfg.Observability.Metrics.Path, gin.WrapH(promhttp.Handler()))
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
		c.JSON(200, gin.H{
			"status": "up",
			"env":    cfg.Environment,
			"mode":   "hybrid (auth=pg, repos=weaviate)",
		})
	})

	log.Info().Str("port", cfg.Server.Port).Msg("Playground ready")
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}
