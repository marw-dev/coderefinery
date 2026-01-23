package integration

import (
	"log"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
	dsn := "postgres://refinery:secret@localhost:5434/coderefinery?sslmode=disable"

	// Override via Env Var möglich
	if envDSN := os.Getenv("TEST_DB_DSN"); envDSN != "" {
		dsn = envDSN
	}

	var err error
	testDB, err = sqlx.Connect("pgx", dsn)
	if err != nil {
		log.Fatalf("Could not connect to test database: %v", err)
	}

	cleanup()
	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

func cleanup() {
	testDB.Exec("TRUNCATE TABLE users, projects, files, code_chunks CASCADE")
}
