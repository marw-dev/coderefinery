package indexer

import (
	"coderefinery/internal/core/domain"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/pgvector/pgvector-go"
)

type DB struct {
	db *sqlx.DB
}

func NewDB(db *sqlx.DB) *DB {
	return &DB{db: db}
}

// GetFileModTime prüft, wann eine Datei in einem Projekt zuletzt indiziert wurde
func (db *DB) GetFileModTime(projectID uuid.UUID, path string) (time.Time, bool) {
	var lastIndexed time.Time
	// Wir suchen nach der Datei im Kontext des Projekts
	query := `SELECT last_indexed FROM files WHERE project_id = $1 AND path = $2`
	err := db.db.Get(&lastIndexed, query, projectID, path)
	if err != nil {
		return time.Time{}, false
	}
	return lastIndexed, true
}

// SaveFileChunks führt ein Upsert für die Datei durch und speichert die Chunks neu
func (db *DB) SaveFileChunks(projectID uuid.UUID, path string, modTime time.Time, chunks []domain.CodeChunk) error {
	tx, err := db.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Datei Upsert (Metadaten aktualisieren oder anlegen)
	var fileID uuid.UUID
	queryFile := `
		INSERT INTO files (project_id, path, last_modified, last_indexed, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW(), NOW())
		ON CONFLICT (project_id, path) DO UPDATE SET
			last_modified = $3,
			last_indexed = NOW(),
			updated_at = NOW()
		RETURNING id
	`
	err = tx.QueryRowx(queryFile, projectID, path, modTime).Scan(&fileID)
	if err != nil {
		return err
	}

	// 2. Alte Chunks für diese Datei löschen (Clean Replace)
	_, err = tx.Exec(`DELETE FROM code_chunks WHERE file_id = $1`, fileID)
	if err != nil {
		return err
	}

	// 3. Neue Chunks einfügen
	if len(chunks) > 0 {
		stmt, err := tx.Preparex(`
			INSERT INTO code_chunks (
				file_id, content, signature, comments,
				start_line, end_line, chunk_type, embedding, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
		`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, c := range chunks {
			// Embedding in pgvector Format konvertieren
			vec := pgvector.NewVector(c.Embedding)
			_, err := stmt.Exec(
				fileID, c.Content, c.Signature, c.Comments,
				c.StartLine, c.EndLine, c.ChunkType, vec,
			)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

// DeleteProjectFiles löscht alle Dateien eines Projekts (Chunks via Cascade)
func (db *DB) DeleteProjectFiles(projectID uuid.UUID) error {
	_, err := db.db.Exec(`DELETE FROM files WHERE project_id = $1`, projectID)
	return err
}

// DeleteAllFileChunks löscht ALLE Vektoren und Chunks aus der Datenbank.
// Wird benötigt, wenn das Embedding-Modell gewechselt wird.
func (db *DB) DeleteAllFileChunks() error {

	_, err := db.db.Exec("DELETE FROM code_chunks")

	return err
}
