package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"coderefinery/internal/config"
	"coderefinery/internal/embedding"
	"coderefinery/internal/indexer"
	"coderefinery/internal/search"
	"coderefinery/internal/server"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "", "Path to config file")
	projectPath := flag.String("path", ".", "Path to project root")
	port := flag.String("port", "8080", "Server port")
	flag.Parse()

	// Load config
	cfg := config.NewDefault()
	if *configPath != "" {
		if err := cfg.LoadFromFile(*configPath); err != nil {
			log.Printf("Warning: Could not load config: %v", err)
		}
	}
	cfg.ProjectPath = *projectPath
	cfg.ServerPort = *port
	
	dbPath := "refinery.db"

	// Initialize dependencies
	ctx := context.Background()

	embedder, err := embedding.NewOllamaEmbedder(cfg.Ollama)
	if err != nil {
		log.Fatalf("Failed to create embedder: %v", err)
	}

	idx, err := indexer.NewIndexer(cfg.Indexer, embedder, dbPath)
    if err != nil { log.Fatalf("Failed to init indexer/db: %v", err) }
    defer idx.Close()

	go func() {
        if err := idx.BuildIndex(ctx, cfg.ProjectPath); err != nil {
            log.Printf("Background indexing error: %v", err)
        } else {
            log.Println("Background sync complete.")
        }
    }()

	searcher := search.NewSearcher(idx)

	// Build initial index
	log.Printf("Indexing project at: %s\n", cfg.ProjectPath)
	start := time.Now()
	
	if err := idx.BuildIndex(ctx, cfg.ProjectPath); err != nil {
		log.Fatalf("Failed to build index: %v", err)
	}
	
	stats := idx.Stats()
	log.Printf("Indexed %d chunks from %d files in %v\n", 
		stats.TotalChunks, stats.TotalFiles, time.Since(start))

	// Start file watcher
	if err := idx.Watch(ctx, cfg.ProjectPath); err != nil {
		log.Printf("File watcher disabled: %v\n", err)
	} else {
		log.Println("File watcher enabled")
	}

	// Start HTTP server
	srv := server.NewServer(cfg.Server, searcher, idx)
	
	go func() {
		log.Printf("Server running on http://localhost:%s\n", cfg.ServerPort)
		if err := srv.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
	
	idx.Close()
	log.Println("Shutdown complete")
}