package indexer

import (
	"database/sql"
	"log"
)

const TargetVersion = 2

type Migration func(*sql.Tx) error

var migrations = []struct {
	Version int
	Apply	Migration
}{
	// v1: Basis-Schema (Das ursprüngliche Schema)
		{
			Version: 1,
			Apply: func(tx *sql.Tx) error {
				log.Println("Applying Migration v1: Initial Schema")
				query := `
				CREATE TABLE IF NOT EXISTS chunks (
					id TEXT PRIMARY KEY,
					file_path TEXT NOT NULL,
					content TEXT NOT NULL,
					signature TEXT,
					comments TEXT,
					start_line INTEGER,
					end_line INTEGER,
					chunk_type TEXT,
					language TEXT,
					embedding BLOB,
					last_modified INTEGER,
					indexed_at INTEGER DEFAULT (strftime('%s', 'now'))
				);
				CREATE INDEX IF NOT EXISTS idx_file_path ON chunks(file_path);
				CREATE INDEX IF NOT EXISTS idx_language ON chunks(language);
				CREATE TABLE IF NOT EXISTS file_metadata (
					file_path TEXT PRIMARY KEY,
					last_modified INTEGER NOT NULL,
					last_indexed INTEGER NOT NULL,
					chunk_count INTEGER DEFAULT 0
				);`
				_, err := tx.Exec(query)
				return err
			},
		},
		// v2: Hinzufügen der 'imports' Spalte
		{
			Version: 2,
			Apply: func(tx *sql.Tx) error {
				log.Println("Applying Migration v2: Add imports column")
				// Prüfen, ob Spalte schon existiert (für den Fall von manuellen Eingriffen)
				// SQLite unterstützt 'IF NOT EXISTS' bei ADD COLUMN erst in neueren Versionen,
				// daher ist ein simpler ALTER TABLE hier meist sicher, wenn Versionierung stimmt.
				query := `ALTER TABLE chunks ADD COLUMN imports TEXT;`
				_, err := tx.Exec(query)
				return err
			},
		},
}
