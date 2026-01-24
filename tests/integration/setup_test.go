package integration

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	// Wir versuchen eine Verbindung zur lokalen DB (aus docker-compose) aufzubauen
	// Falls du 'dockertest' nutzen willst, wäre das Code hier umfangreicher.
	// Für jetzt nehmen wir an: Docker Compose läuft ("postgres" Container auf Port 5434).

	dsn := "postgres://refinery:secret@localhost:5434/coderefinery?sslmode=disable"

	// Retry Loop, falls DB noch bootet
	var err error
	for range 10 {
		testDB, err = sqlx.Connect("postgres", dsn)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if err != nil {
		log.Printf("Could not connect to test DB at %s: %v. Make sure docker-compose is running!", dsn, err)
		os.Exit(1)
	}

	// WICHTIG: Schema initialisieren
	if err := applySchema(testDB); err != nil {
		log.Fatalf("Failed to apply schema: %v", err)
	}

	code := m.Run()

	cleanup()
	testDB.Close()
	os.Exit(code)
}

func applySchema(db *sqlx.DB) error {
	// Pfad zur Migration Datei finden (relativ von tests/integration)
	path := "../../migrations/001_initial_schema.up.sql"
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("could not read migration file at %s: %w", path, err)
	}

	// SQL ausführen
	_, err = db.Exec(string(content))
	return err
}

func cleanup() {
	if testDB != nil {
		tables := []string{"repositories", "users"}
		for _, t := range tables {
			_, _ = testDB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", t))
		}
	}
}
