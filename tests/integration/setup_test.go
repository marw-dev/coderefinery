package integration

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/weaviate/weaviate-go-client/v4/weaviate"
)

var (
	testDB       *sqlx.DB
	testWeaviate *weaviate.Client
)

const (
	maxRetries     = 10
	retryDelay     = 1 * time.Second
	weaviatePort   = "8090"
	postgresPort   = "5434"
	testTimeout    = 30 * time.Second
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// 1. Setup Postgres
	if err := setupPostgres(ctx); err != nil {
		log.Fatalf("Failed to setup Postgres: %v", err)
	}

	// 2. Setup Weaviate
	if err := setupWeaviate(ctx); err != nil {
		log.Fatalf("Failed to setup Weaviate: %v", err)
	}

	// 3. Initial Schema Setup
	if err := initializeSchema(); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	// 4. Run Tests
	code := m.Run()

	// 5. Cleanup
	cleanup()
	if testDB != nil {
		testDB.Close()
	}

	os.Exit(code)
}

// setupPostgres initialisiert die Postgres Verbindung mit Retry-Logik
func setupPostgres(ctx context.Context) error {
	dsn := fmt.Sprintf("postgres://refinery:secret@localhost:%s/coderefinery?sslmode=disable", postgresPort)

	var err error
	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		testDB, err = sqlx.Connect("postgres", dsn)
		if err == nil {
			// Test connection
			if err = testDB.Ping(); err == nil {
				log.Printf("Connected to Postgres on port %s", postgresPort)

				// Connection Pool Settings
				testDB.SetMaxOpenConns(10)
				testDB.SetMaxIdleConns(5)
				testDB.SetConnMaxLifetime(5 * time.Minute)

				return nil
			}
			testDB.Close()
		}

		log.Printf("Waiting for Postgres (attempt %d/%d)...", i+1, maxRetries)
		time.Sleep(retryDelay)
	}

	return fmt.Errorf("could not connect to Postgres after %d attempts: %w", maxRetries, err)
}

// setupWeaviate initialisiert den Weaviate Client mit Retry-Logik
func setupWeaviate(ctx context.Context) error {
	wCfg := weaviate.Config{
		Host:   fmt.Sprintf("localhost:%s", weaviatePort),
		Scheme: "http",
		ConnectionClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	var err error
	testWeaviate, err = weaviate.NewClient(wCfg)
	if err != nil {
		return fmt.Errorf("failed to create Weaviate client: %w", err)
	}

	// Warte bis Weaviate bereit ist
	for i := range maxRetries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ready, err := testWeaviate.Misc().ReadyChecker().Do(context.Background())
		if err == nil && ready {
			log.Printf("Connected to Weaviate on port %s", weaviatePort)
			return nil
		}

		log.Printf("Waiting for Weaviate (attempt %d/%d)...", i+1, maxRetries)
		time.Sleep(retryDelay)
	}

	return fmt.Errorf("weaviate not ready after %d attempts", maxRetries)
}

// initializeSchema wendet die initialen Postgres Migrationen an
func initializeSchema() error {
	// Pfad zur Migration Datei
	migrationPath := "../../migrations/001_initial_schema.up.sql"

	content, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("could not read migration file at %s: %w", migrationPath, err)
	}

	// Drop und neu erstellen für sauberen Zustand
	_, err = testDB.Exec(`
		DROP TABLE IF EXISTS users CASCADE;
		DROP TABLE IF EXISTS repositories CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("failed to drop existing tables: %w", err)
	}

	// SQL Migration ausführen
	_, err = testDB.Exec(string(content))
	if err != nil {
		return fmt.Errorf("failed to apply migration: %w", err)
	}

	log.Println("Initialized database schema")
	return nil
}

// cleanup bereinigt Testdaten nach jedem Test
func cleanup() {
	cleanupPostgres()
	cleanupWeaviate()
}

// cleanupPostgres löscht alle Daten aus Postgres Tabellen
func cleanupPostgres() {
	if testDB == nil {
		return
	}

	tables := []string{"users"}

	for _, table := range tables {
		_, err := testDB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if err != nil {
			log.Printf("Warning: Failed to truncate table %s: %v", table, err)
		}
	}
}

// cleanupWeaviate löscht alle Test-Klassen aus Weaviate
func cleanupWeaviate() {
	if testWeaviate == nil {
		return
	}

	ctx := context.Background()
	classes := []string{"Repository", "TestCodeChunk", "CodeChunk"}

	for _, className := range classes {
		// Prüfen ob Klasse existiert
		exists, err := testWeaviate.Schema().ClassExistenceChecker().
			WithClassName(className).
			Do(ctx)

		if err != nil {
			log.Printf("Warning: Failed to check class %s: %v", className, err)
			continue
		}

		if exists {
			err = testWeaviate.Schema().ClassDeleter().
				WithClassName(className).
				Do(ctx)

			if err != nil {
				log.Printf("Warning: Failed to delete class %s: %v", className, err)
			}
		}
	}

	// Kurz warten damit Weaviate aufräumen kann
	time.Sleep(100 * time.Millisecond)
}

// Helper Functions für Tests

// WaitForConsistency wartet bis Weaviate eventual consistency erreicht hat
func WaitForConsistency() {
	time.Sleep(150 * time.Millisecond)
}

// RequireWeaviateClass prüft ob eine Weaviate Klasse existiert
func RequireWeaviateClass(t *testing.T, className string) {
	exists, err := testWeaviate.Schema().ClassExistenceChecker().
		WithClassName(className).
		Do(context.Background())

	if err != nil {
		t.Fatalf("Failed to check class %s: %v", className, err)
	}

	if !exists {
		t.Fatalf("Class %s does not exist", className)
	}
}

// GetWeaviateStats gibt Debug-Informationen über Weaviate zurück
func GetWeaviateStats(className string) (map[string]interface{}, error) {
	meta, err := testWeaviate.Misc().MetaGetter().Do(context.Background())
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"version": meta.Version,
	}, nil
}
